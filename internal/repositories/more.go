package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pulse/chat-service/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AttachmentRepo struct{ DB *gorm.DB }

func (r *AttachmentRepo) Create(ctx context.Context, a *models.Attachment) error {
	return r.DB.WithContext(ctx).Create(a).Error
}

func (r *AttachmentRepo) ListByMessageIDs(ctx context.Context, ids []uuid.UUID) ([]models.Attachment, error) {
	var items []models.Attachment
	if len(ids) == 0 {
		return items, nil
	}
	err := r.DB.WithContext(ctx).Where("message_id IN ?", ids).Find(&items).Error
	return items, err
}

type ReactionRepo struct{ DB *gorm.DB }

func (r *ReactionRepo) Upsert(ctx context.Context, reaction *models.Reaction) error {
	return r.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "message_id"}, {Name: "user_id"}, {Name: "emoji"}},
		DoNothing: true,
	}).Create(reaction).Error
}

func (r *ReactionRepo) Delete(ctx context.Context, messageID, userID uuid.UUID, emoji string) error {
	return r.DB.WithContext(ctx).Where("message_id = ? AND user_id = ? AND emoji = ?", messageID, userID, emoji).
		Delete(&models.Reaction{}).Error
}

func (r *ReactionRepo) ListByMessageIDs(ctx context.Context, ids []uuid.UUID) ([]models.Reaction, error) {
	var items []models.Reaction
	if len(ids) == 0 {
		return items, nil
	}
	err := r.DB.WithContext(ctx).Where("message_id IN ?", ids).Find(&items).Error
	return items, err
}

type ReceiptRepo struct{ DB *gorm.DB }

func (r *ReceiptRepo) Upsert(ctx context.Context, receipt *models.ReadReceipt) error {
	return r.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "message_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"status", "delivered_at", "read_at", "seen_at", "updated_at"}),
	}).Create(receipt).Error
}

type PinRepo struct{ DB *gorm.DB }

func (r *PinRepo) Pin(ctx context.Context, p *models.PinnedMessage) error {
	return r.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(p).Error
}

func (r *PinRepo) Unpin(ctx context.Context, conversationID, messageID uuid.UUID) error {
	return r.DB.WithContext(ctx).Where("conversation_id = ? AND message_id = ?", conversationID, messageID).
		Delete(&models.PinnedMessage{}).Error
}

func (r *PinRepo) List(ctx context.Context, conversationID uuid.UUID) ([]models.PinnedMessage, error) {
	var items []models.PinnedMessage
	err := r.DB.WithContext(ctx).Where("conversation_id = ?", conversationID).Order("created_at DESC").Find(&items).Error
	return items, err
}

type BlockRepo struct{ DB *gorm.DB }

func (r *BlockRepo) Block(ctx context.Context, b *models.BlockedUser) error {
	return r.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(b).Error
}

func (r *BlockRepo) Unblock(ctx context.Context, blocker, blocked uuid.UUID) error {
	return r.DB.WithContext(ctx).Where("blocker_id = ? AND blocked_id = ?", blocker, blocked).
		Delete(&models.BlockedUser{}).Error
}

func (r *BlockRepo) IsBlocked(ctx context.Context, a, b uuid.UUID) (bool, error) {
	var count int64
	err := r.DB.WithContext(ctx).Model(&models.BlockedUser{}).
		Where("(blocker_id = ? AND blocked_id = ?) OR (blocker_id = ? AND blocked_id = ?)", a, b, b, a).
		Count(&count).Error
	return count > 0, err
}

func (r *BlockRepo) List(ctx context.Context, blocker uuid.UUID) ([]models.BlockedUser, error) {
	var items []models.BlockedUser
	err := r.DB.WithContext(ctx).Where("blocker_id = ?", blocker).Find(&items).Error
	return items, err
}

type PresenceRepo struct{ DB *gorm.DB }

func (r *PresenceRepo) Upsert(ctx context.Context, p *models.UserPresence) error {
	return r.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"status", "device", "last_seen_at", "updated_at"}),
	}).Create(p).Error
}

func (r *PresenceRepo) Get(ctx context.Context, userID uuid.UUID) (*models.UserPresence, error) {
	var p models.UserPresence
	err := r.DB.WithContext(ctx).Where("user_id = ?", userID).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

type DraftRepo struct{ DB *gorm.DB }

func (r *DraftRepo) Upsert(ctx context.Context, d *models.DraftMessage) error {
	return r.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "conversation_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"body", "reply_to_id", "updated_at"}),
	}).Create(d).Error
}

func (r *DraftRepo) Get(ctx context.Context, conversationID, userID uuid.UUID) (*models.DraftMessage, error) {
	var d models.DraftMessage
	err := r.DB.WithContext(ctx).Where("conversation_id = ? AND user_id = ?", conversationID, userID).First(&d).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DraftRepo) Delete(ctx context.Context, conversationID, userID uuid.UUID) error {
	return r.DB.WithContext(ctx).Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Delete(&models.DraftMessage{}).Error
}

type SettingsRepo struct{ DB *gorm.DB }

func (r *SettingsRepo) GetOrCreate(ctx context.Context, userID uuid.UUID) (*models.ChatSettings, error) {
	var s models.ChatSettings
	err := r.DB.WithContext(ctx).Where("user_id = ?", userID).First(&s).Error
	if err == gorm.ErrRecordNotFound {
		s = models.ChatSettings{
			UserID:              userID,
			Theme:               "system",
			NotificationEnabled: true,
			SoundEnabled:        true,
			ReadReceiptsEnabled: true,
			LastSeenVisible:     true,
			Language:            "en",
		}
		if err := r.DB.WithContext(ctx).Create(&s).Error; err != nil {
			return nil, err
		}
		return &s, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SettingsRepo) Save(ctx context.Context, s *models.ChatSettings) error {
	return r.DB.WithContext(ctx).Save(s).Error
}

type NotificationRepo struct{ DB *gorm.DB }

func (r *NotificationRepo) Create(ctx context.Context, n *models.NotificationQueue) error {
	return r.DB.WithContext(ctx).Create(n).Error
}

func (r *NotificationRepo) List(ctx context.Context, userID uuid.UUID, limit int) ([]models.NotificationQueue, error) {
	var items []models.NotificationQueue
	err := r.DB.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&items).Error
	return items, err
}

type CallRepo struct{ DB *gorm.DB }

func (r *CallRepo) Create(ctx context.Context, c *models.CallHistory) error {
	return r.DB.WithContext(ctx).Create(c).Error
}

func (r *CallRepo) Update(ctx context.Context, c *models.CallHistory) error {
	return r.DB.WithContext(ctx).Save(c).Error
}

func (r *CallRepo) List(ctx context.Context, userID uuid.UUID, limit int) ([]models.CallHistory, error) {
	var items []models.CallHistory
	err := r.DB.WithContext(ctx).
		Joins("INNER JOIN conversation_members cm ON cm.conversation_id = call_histories.conversation_id AND cm.user_id = ?", userID).
		Order("call_histories.created_at DESC").Limit(limit).Find(&items).Error
	return items, err
}

type DeviceRepo struct{ DB *gorm.DB }

func (r *DeviceRepo) Upsert(ctx context.Context, d *models.DeviceToken) error {
	return r.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "token"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_id", "platform", "active", "p256dh", "auth", "updated_at"}),
	}).Create(d).Error
}

func (r *DeviceRepo) Deactivate(ctx context.Context, userID uuid.UUID, endpoint string) error {
	return r.DB.WithContext(ctx).Model(&models.DeviceToken{}).
		Where("user_id = ? AND token = ?", userID, endpoint).
		Updates(map[string]interface{}{"active": false, "updated_at": time.Now().UTC()}).Error
}

func (r *DeviceRepo) ListWeb(ctx context.Context, userID uuid.UUID) ([]models.DeviceToken, error) {
	var items []models.DeviceToken
	err := r.DB.WithContext(ctx).Where("user_id = ? AND active = true AND platform = ?", userID, "web").Find(&items).Error
	return items, err
}

type VoiceRepo struct{ DB *gorm.DB }

func (r *VoiceRepo) Create(ctx context.Context, v *models.VoiceMessage) error {
	return r.DB.WithContext(ctx).Create(v).Error
}

type DeletedRepo struct{ DB *gorm.DB }

func (r *DeletedRepo) Create(ctx context.Context, d *models.DeletedMessage) error {
	return r.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(d).Error
}

type EditRepo struct{ DB *gorm.DB }

func (r *EditRepo) Create(ctx context.Context, e *models.MessageEdit) error {
	return r.DB.WithContext(ctx).Create(e).Error
}

type MentionRepo struct{ DB *gorm.DB }

func (r *MentionRepo) CreateMany(ctx context.Context, mentions []models.Mention) error {
	if len(mentions) == 0 {
		return nil
	}
	return r.DB.WithContext(ctx).Create(&mentions).Error
}

type StarRepo struct{ DB *gorm.DB }

func (r *StarRepo) Star(ctx context.Context, s *models.StarredMessage) error {
	return r.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(s).Error
}

func (r *StarRepo) Unstar(ctx context.Context, messageID, userID uuid.UUID) error {
	return r.DB.WithContext(ctx).Where("message_id = ? AND user_id = ?", messageID, userID).
		Delete(&models.StarredMessage{}).Error
}

func (r *StarRepo) List(ctx context.Context, userID uuid.UUID, limit int) ([]models.StarredMessage, error) {
	var items []models.StarredMessage
	err := r.DB.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&items).Error
	return items, err
}

type TypingRepo struct{ DB *gorm.DB }

func (r *TypingRepo) Upsert(ctx context.Context, t *models.TypingStatus) error {
	return r.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "conversation_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"is_typing", "is_recording", "expires_at", "updated_at"}),
	}).Create(t).Error
}
