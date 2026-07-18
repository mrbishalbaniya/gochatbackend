package repositories

import (
	"context"
	"strings"
	"time"

	"github.com/pulse/chat-service/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserRepo struct{ DB *gorm.DB }

func (r *UserRepo) Create(ctx context.Context, u *models.User) error {
	return r.DB.WithContext(ctx).Create(u).Error
}

func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	err := r.DB.WithContext(ctx).Where("LOWER(email) = ?", strings.ToLower(email)).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	var u models.User
	err := r.DB.WithContext(ctx).Where("LOWER(username) = ?", strings.ToLower(username)).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) FindByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var u models.User
	err := r.DB.WithContext(ctx).First(&u, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) Update(ctx context.Context, u *models.User) error {
	return r.DB.WithContext(ctx).Save(u).Error
}

func (r *UserRepo) Search(ctx context.Context, q string, limit int) ([]models.User, error) {
	var users []models.User
	like := "%" + strings.ToLower(q) + "%"
	err := r.DB.WithContext(ctx).
		Where("LOWER(username) LIKE ? OR LOWER(display_name) LIKE ? OR LOWER(email) LIKE ?", like, like, like).
		Limit(limit).Find(&users).Error
	return users, err
}

func (r *UserRepo) FindByUsernames(ctx context.Context, names []string) ([]models.User, error) {
	var users []models.User
	lower := make([]string, len(names))
	for i, n := range names {
		lower[i] = strings.ToLower(n)
	}
	err := r.DB.WithContext(ctx).Where("LOWER(username) IN ?", lower).Find(&users).Error
	return users, err
}

type RefreshRepo struct{ DB *gorm.DB }

func (r *RefreshRepo) Create(ctx context.Context, t *models.RefreshToken) error {
	return r.DB.WithContext(ctx).Create(t).Error
}

func (r *RefreshRepo) FindValid(ctx context.Context, hash string) (*models.RefreshToken, error) {
	var t models.RefreshToken
	err := r.DB.WithContext(ctx).
		Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", hash, time.Now().UTC()).
		First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *RefreshRepo) Revoke(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	return r.DB.WithContext(ctx).Model(&models.RefreshToken{}).Where("id = ?", id).Update("revoked_at", now).Error
}

type ConversationRepo struct{ DB *gorm.DB }

func (r *ConversationRepo) Create(ctx context.Context, c *models.Conversation, members []models.ConversationMember) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(c).Error; err != nil {
			return err
		}
		for i := range members {
			members[i].ConversationID = c.ID
			if err := tx.Create(&members[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *ConversationRepo) FindByID(ctx context.Context, id uuid.UUID) (*models.Conversation, error) {
	var c models.Conversation
	err := r.DB.WithContext(ctx).First(&c, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ConversationRepo) IsMember(ctx context.Context, conversationID, userID uuid.UUID) (bool, error) {
	var count int64
	err := r.DB.WithContext(ctx).Model(&models.ConversationMember{}).
		Where("conversation_id = ? AND user_id = ? AND left_at IS NULL", conversationID, userID).
		Count(&count).Error
	return count > 0, err
}

func (r *ConversationRepo) GetMember(ctx context.Context, conversationID, userID uuid.UUID) (*models.ConversationMember, error) {
	var m models.ConversationMember
	err := r.DB.WithContext(ctx).
		Where("conversation_id = ? AND user_id = ? AND left_at IS NULL", conversationID, userID).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *ConversationRepo) ListForUser(ctx context.Context, userID uuid.UUID, archived bool, limit int) ([]models.Conversation, map[uuid.UUID]models.ConversationMember, error) {
	var members []models.ConversationMember
	q := r.DB.WithContext(ctx).Where("user_id = ? AND left_at IS NULL AND is_archived = ?", userID, archived).
		Order("updated_at DESC").Limit(limit)
	if err := q.Find(&members).Error; err != nil {
		return nil, nil, err
	}
	ids := make([]uuid.UUID, 0, len(members))
	memberMap := map[uuid.UUID]models.ConversationMember{}
	for _, m := range members {
		ids = append(ids, m.ConversationID)
		memberMap[m.ConversationID] = m
	}
	if len(ids) == 0 {
		return []models.Conversation{}, memberMap, nil
	}
	var convs []models.Conversation
	err := r.DB.WithContext(ctx).Where("id IN ?", ids).
		Order("COALESCE(last_message_at, created_at) DESC").Find(&convs).Error
	return convs, memberMap, err
}

func (r *ConversationRepo) FindDirect(ctx context.Context, a, b uuid.UUID) (*models.Conversation, error) {
	var conv models.Conversation
	err := r.DB.WithContext(ctx).Raw(`
		SELECT c.* FROM conversations c
		INNER JOIN conversation_members m1 ON m1.conversation_id = c.id AND m1.user_id = ? AND m1.left_at IS NULL AND m1.deleted_at IS NULL
		INNER JOIN conversation_members m2 ON m2.conversation_id = c.id AND m2.user_id = ? AND m2.left_at IS NULL AND m2.deleted_at IS NULL
		WHERE c.type = 'direct' AND c.deleted_at IS NULL
		LIMIT 1
	`, a, b).Scan(&conv).Error
	if err != nil {
		return nil, err
	}
	if conv.ID == uuid.Nil {
		return nil, gorm.ErrRecordNotFound
	}
	return &conv, nil
}

func (r *ConversationRepo) ListMembers(ctx context.Context, conversationID uuid.UUID) ([]models.ConversationMember, error) {
	var members []models.ConversationMember
	err := r.DB.WithContext(ctx).Where("conversation_id = ? AND left_at IS NULL", conversationID).Find(&members).Error
	return members, err
}

func (r *ConversationRepo) UpdateMember(ctx context.Context, m *models.ConversationMember) error {
	return r.DB.WithContext(ctx).Save(m).Error
}

func (r *ConversationRepo) AddMember(ctx context.Context, m *models.ConversationMember) error {
	return r.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(m).Error
}

func (r *ConversationRepo) TouchLastMessage(ctx context.Context, id uuid.UUID, at time.Time) error {
	return r.DB.WithContext(ctx).Model(&models.Conversation{}).Where("id = ?", id).
		Updates(map[string]interface{}{"last_message_at": at, "updated_at": at}).Error
}

type MessageRepo struct{ DB *gorm.DB }

func (r *MessageRepo) Create(ctx context.Context, m *models.Message) error {
	return r.DB.WithContext(ctx).Create(m).Error
}

func (r *MessageRepo) FindByID(ctx context.Context, id uuid.UUID) (*models.Message, error) {
	var m models.Message
	err := r.DB.WithContext(ctx).First(&m, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *MessageRepo) List(ctx context.Context, conversationID uuid.UUID, before *time.Time, limit int, deletedFor uuid.UUID) ([]models.Message, error) {
	q := r.DB.WithContext(ctx).Where("conversation_id = ?", conversationID).
		Where("id NOT IN (SELECT message_id FROM deleted_messages WHERE user_id = ? AND deleted_at IS NULL)", deletedFor)
	if before != nil {
		q = q.Where("created_at < ?", *before)
	}
	var msgs []models.Message
	err := q.Order("created_at DESC").Limit(limit).Find(&msgs).Error
	return msgs, err
}

func (r *MessageRepo) Update(ctx context.Context, m *models.Message) error {
	return r.DB.WithContext(ctx).Save(m).Error
}

func (r *MessageRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return r.DB.WithContext(ctx).Delete(&models.Message{}, "id = ?", id).Error
}

func (r *MessageRepo) Search(ctx context.Context, userID uuid.UUID, q string, conversationID *uuid.UUID, msgType string, limit int) ([]models.Message, error) {
	like := "%" + strings.ToLower(q) + "%"
	db := r.DB.WithContext(ctx).Table("messages m").
		Joins("INNER JOIN conversation_members cm ON cm.conversation_id = m.conversation_id AND cm.user_id = ? AND cm.left_at IS NULL AND cm.deleted_at IS NULL", userID).
		Where("m.deleted_at IS NULL AND LOWER(m.body) LIKE ?", like)
	if conversationID != nil {
		db = db.Where("m.conversation_id = ?", *conversationID)
	}
	if msgType != "" {
		db = db.Where("m.type = ?", msgType)
	}
	var msgs []models.Message
	err := db.Select("m.*").Order("m.created_at DESC").Limit(limit).Scan(&msgs).Error
	return msgs, err
}

func (r *MessageRepo) Latest(ctx context.Context, conversationID uuid.UUID) (*models.Message, error) {
	var m models.Message
	err := r.DB.WithContext(ctx).Where("conversation_id = ?", conversationID).Order("created_at DESC").First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}
