package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/pulse/chat-service/internal/constants"
	"github.com/pulse/chat-service/internal/models"
	"github.com/pulse/chat-service/internal/utils"
	"github.com/google/uuid"
)

func (s *Services) ClearHistory(ctx context.Context, userID, conversationID uuid.UUID) error {
	if _, err := s.requireMember(ctx, conversationID, userID); err != nil {
		return err
	}
	var msgs []models.Message
	if err := s.DB.WithContext(ctx).Where("conversation_id = ?", conversationID).Select("id").Find(&msgs).Error; err != nil {
		return err
	}
	if len(msgs) == 0 {
		return nil
	}
	rows := make([]models.DeletedMessage, 0, len(msgs))
	for _, m := range msgs {
		rows = append(rows, models.DeletedMessage{MessageID: m.ID, UserID: userID, Scope: "me"})
	}
	if err := s.DB.WithContext(ctx).CreateInBatches(&rows, 100).Error; err != nil {
		return err
	}
	s.publish(ctx, constants.WSEventConversation, &conversationID, &userID, map[string]interface{}{
		"action": "cleared", "userId": userID,
	})
	return nil
}

func (s *Services) IssueWSTicket(ctx context.Context, userID, conversationID uuid.UUID) (string, error) {
	if _, err := s.requireMember(ctx, conversationID, userID); err != nil {
		return "", err
	}
	raw := userID.String() + ":" + conversationID.String()
	token, err := utils.RandomToken(16)
	if err != nil {
		return "", err
	}
	key := "chat:ws-ticket:" + token
	if err := s.RDB.Set(ctx, key, raw, 5*time.Minute).Err(); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Services) ConsumeWSTicket(ctx context.Context, ticket string) (userID, conversationID uuid.UUID, err error) {
	key := "chat:ws-ticket:" + ticket
	val, err := s.RDB.GetDel(ctx, key).Result()
	if err != nil {
		return uuid.Nil, uuid.Nil, errors.New("invalid ticket")
	}
	parts := strings.Split(val, ":")
	if len(parts) < 2 {
		return uuid.Nil, uuid.Nil, errors.New("invalid ticket")
	}
	userID, err = uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	conversationID, err = uuid.Parse(parts[1])
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return userID, conversationID, nil
}

func (s *Services) MarkDeliveredOnJoin(ctx context.Context, userID, conversationID uuid.UUID) {
	var msgs []models.Message
	_ = s.DB.WithContext(ctx).
		Where("conversation_id = ? AND sender_id <> ?", conversationID, userID).
		Order("created_at DESC").Limit(50).Find(&msgs).Error
	ids := make([]uuid.UUID, 0, len(msgs))
	for _, m := range msgs {
		ids = append(ids, m.ID)
	}
	if len(ids) == 0 {
		return
	}
	_ = s.MarkReceipts(ctx, userID, conversationID, ids, constants.StatusDelivered)
}
