package services

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/pulse/chat-service/internal/constants"
	"github.com/pulse/chat-service/internal/dto"
	"github.com/pulse/chat-service/internal/models"
	"github.com/pulse/chat-service/internal/utils"
	"gorm.io/datatypes"
)

type MessageView struct {
	Message     models.Message      `json:"message"`
	Attachments []models.Attachment `json:"attachments,omitempty"`
	Reactions   []models.Reaction   `json:"reactions,omitempty"`
	Voice       *models.VoiceMessage `json:"voice,omitempty"`
}

func (s *Services) SendMessage(ctx context.Context, userID, conversationID uuid.UUID, req dto.SendMessageRequest) (*MessageView, error) {
	if _, err := s.requireMember(ctx, conversationID, userID); err != nil {
		return nil, err
	}
	body := utils.SanitizeText(req.Body, 10000)
	if body == "" && req.Type == constants.MessageText {
		return nil, errors.New("message body required")
	}
	msg := &models.Message{
		ConversationID: conversationID,
		SenderID:       userID,
		Type:           req.Type,
		Body:           body,
		ReplyToID:      req.ReplyToID,
		ClientMsgID:    utils.SanitizeText(req.ClientMsgID, 64),
		Status:         constants.StatusSent,
		Metadata:       jsonMeta(req.Metadata),
		ScheduledAt:    req.ScheduledAt,
	}
	if err := s.Messages.Create(ctx, msg); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	_ = s.Conversations.TouchLastMessage(ctx, conversationID, now)
	_ = s.Drafts.Delete(ctx, conversationID, userID)

	usernames := utils.ExtractMentions(body)
	if len(usernames) > 0 {
		users, _ := s.Users.FindByUsernames(ctx, usernames)
		mentions := make([]models.Mention, 0, len(users))
		for _, u := range users {
			mentions = append(mentions, models.Mention{
				MessageID: msg.ID, ConversationID: conversationID, MentionedUser: u.ID, MentionedBy: userID,
			})
			_ = s.Notifications.Create(ctx, &models.NotificationQueue{
				UserID: u.ID, Type: "mention", Title: "You were mentioned", Body: body, Status: "pending",
				Payload: mustJSON(map[string]interface{}{"conversationId": conversationID.String(), "messageId": msg.ID.String()}),
			})
		}
		_ = s.Mentions.CreateMany(ctx, mentions)
	}

	members, _ := s.Conversations.ListMembers(ctx, conversationID)
	for _, m := range members {
		if m.UserID == userID {
			continue
		}
		delivered := now
		_ = s.Receipts.Upsert(ctx, &models.ReadReceipt{
			MessageID: msg.ID, ConversationID: conversationID, UserID: m.UserID,
			Status: constants.StatusSent, DeliveredAt: &delivered,
		})
		if !m.IsMuted {
			_ = s.Notifications.Create(ctx, &models.NotificationQueue{
				UserID: m.UserID, Type: "message", Title: "New message", Body: truncate(body, 120), Status: "pending",
				Payload: mustJSON(map[string]interface{}{"conversationId": conversationID.String(), "messageId": msg.ID.String()}),
			})
		}
	}

	view := &MessageView{Message: *msg}
	s.publish(ctx, constants.WSEventMessage, &conversationID, &userID, view)
	// Duo-compatible ack for optimistic clients
	s.publish(ctx, "message_ack", &conversationID, &userID, map[string]interface{}{
		"id": msg.ID, "client_temp_id": msg.ClientMsgID, "status": constants.StatusSent, "message": view,
	})
	return view, nil
}

func (s *Services) ListMessages(ctx context.Context, userID, conversationID uuid.UUID, before *time.Time, limit int) ([]MessageView, dto.PaginationMeta, error) {
	if _, err := s.requireMember(ctx, conversationID, userID); err != nil {
		return nil, dto.PaginationMeta{}, err
	}
	if limit <= 0 || limit > 100 {
		limit = 40
	}
	msgs, err := s.Messages.List(ctx, conversationID, before, limit+1, userID)
	if err != nil {
		return nil, dto.PaginationMeta{}, err
	}
	hasMore := len(msgs) > limit
	if hasMore {
		msgs = msgs[:limit]
	}
	ids := make([]uuid.UUID, len(msgs))
	for i, m := range msgs {
		ids[i] = m.ID
	}
	atts, _ := s.Attachments.ListByMessageIDs(ctx, ids)
	reacts, _ := s.Reactions.ListByMessageIDs(ctx, ids)
	attMap := map[uuid.UUID][]models.Attachment{}
	reactMap := map[uuid.UUID][]models.Reaction{}
	for _, a := range atts {
		attMap[a.MessageID] = append(attMap[a.MessageID], a)
	}
	for _, r := range reacts {
		reactMap[r.MessageID] = append(reactMap[r.MessageID], r)
	}
	views := make([]MessageView, 0, len(msgs))
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		views = append(views, MessageView{Message: m, Attachments: attMap[m.ID], Reactions: reactMap[m.ID]})
	}
	meta := dto.PaginationMeta{HasMore: hasMore, Limit: limit}
	if hasMore && len(msgs) > 0 {
		meta.NextCursor = msgs[len(msgs)-1].CreatedAt.UTC().Format(time.RFC3339Nano)
	}
	return views, meta, nil
}

func (s *Services) EditMessage(ctx context.Context, userID, messageID uuid.UUID, body string) (*MessageView, error) {
	msg, err := s.Messages.FindByID(ctx, messageID)
	if err != nil {
		return nil, err
	}
	if msg.SenderID != userID {
		return nil, errors.New("forbidden")
	}
	if _, err := s.requireMember(ctx, msg.ConversationID, userID); err != nil {
		return nil, err
	}
	old := msg.Body
	msg.Body = utils.SanitizeText(body, 10000)
	now := time.Now().UTC()
	msg.IsEdited = true
	msg.EditedAt = &now
	if err := s.Messages.Update(ctx, msg); err != nil {
		return nil, err
	}
	_ = s.Edits.Create(ctx, &models.MessageEdit{MessageID: msg.ID, EditorID: userID, OldBody: old, NewBody: msg.Body})
	view := &MessageView{Message: *msg}
	s.publish(ctx, constants.WSEventMessageEdit, &msg.ConversationID, &userID, view)
	return view, nil
}

func (s *Services) DeleteMessage(ctx context.Context, userID, messageID uuid.UUID, forEveryone bool) error {
	msg, err := s.Messages.FindByID(ctx, messageID)
	if err != nil {
		return err
	}
	if _, err := s.requireMember(ctx, msg.ConversationID, userID); err != nil {
		return err
	}
	if forEveryone {
		if msg.SenderID != userID {
			return errors.New("forbidden")
		}
		if err := s.Messages.SoftDelete(ctx, messageID); err != nil {
			return err
		}
		_ = s.Deleted.Create(ctx, &models.DeletedMessage{MessageID: messageID, UserID: userID, Scope: "everyone"})
		s.publish(ctx, constants.WSEventMessageDelete, &msg.ConversationID, &userID, map[string]interface{}{
			"messageId": messageID, "scope": "everyone",
		})
		return nil
	}
	_ = s.Deleted.Create(ctx, &models.DeletedMessage{MessageID: messageID, UserID: userID, Scope: "me"})
	s.publish(ctx, constants.WSEventMessageDelete, &msg.ConversationID, &userID, map[string]interface{}{
		"messageId": messageID, "scope": "me", "userId": userID,
	})
	return nil
}

func (s *Services) React(ctx context.Context, userID, messageID uuid.UUID, emoji string, remove bool) error {
	msg, err := s.Messages.FindByID(ctx, messageID)
	if err != nil {
		return err
	}
	if _, err := s.requireMember(ctx, msg.ConversationID, userID); err != nil {
		return err
	}
	emoji = utils.SanitizeText(emoji, 32)
	if remove {
		if err := s.Reactions.Delete(ctx, messageID, userID, emoji); err != nil {
			return err
		}
	} else {
		if err := s.Reactions.Upsert(ctx, &models.Reaction{MessageID: messageID, UserID: userID, Emoji: emoji}); err != nil {
			return err
		}
	}
	s.publish(ctx, constants.WSEventReaction, &msg.ConversationID, &userID, map[string]interface{}{
		"messageId": messageID, "emoji": emoji, "removed": remove, "userId": userID,
	})
	return nil
}

func (s *Services) MarkReceipts(ctx context.Context, userID, conversationID uuid.UUID, messageIDs []uuid.UUID, status string) error {
	if _, err := s.requireMember(ctx, conversationID, userID); err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, mid := range messageIDs {
		r := &models.ReadReceipt{MessageID: mid, ConversationID: conversationID, UserID: userID, Status: status}
		switch status {
		case constants.StatusDelivered:
			r.DeliveredAt = &now
		case constants.StatusRead, constants.StatusSeen:
			r.ReadAt = &now
			r.SeenAt = &now
		}
		_ = s.Receipts.Upsert(ctx, r)
	}
	if m, err := s.Conversations.GetMember(ctx, conversationID, userID); err == nil && len(messageIDs) > 0 {
		last := messageIDs[len(messageIDs)-1]
		m.LastReadMsgID = &last
		m.LastReadAt = &now
		m.UnreadCount = 0
		_ = s.Conversations.UpdateMember(ctx, m)
	}
	s.publish(ctx, constants.WSEventReceipt, &conversationID, &userID, map[string]interface{}{
		"messageIds": messageIDs, "status": status, "userId": userID,
	})
	return nil
}

func (s *Services) SetTyping(ctx context.Context, userID, conversationID uuid.UUID, typing, recording bool) error {
	if _, err := s.requireMember(ctx, conversationID, userID); err != nil {
		return err
	}
	exp := time.Now().UTC().Add(5 * time.Second)
	_ = s.Typing.Upsert(ctx, &models.TypingStatus{
		ConversationID: conversationID, UserID: userID, IsTyping: typing, IsRecording: recording, ExpiresAt: exp,
	})
	key := constants.RedisTypingKey + conversationID.String()
	_ = s.RDB.Set(ctx, key+":"+userID.String(), "1", 5*time.Second).Err()
	evtType := constants.WSEventTyping
	if recording {
		evtType = constants.WSEventRecording
	}
	s.publish(ctx, evtType, &conversationID, &userID, map[string]interface{}{
		"userId": userID, "isTyping": typing, "isRecording": recording,
	})
	return nil
}

func (s *Services) SetPresence(ctx context.Context, userID uuid.UUID, status, device string) error {
	switch status {
	case constants.PresenceOnline, constants.PresenceOffline, constants.PresenceAway,
		constants.PresenceBusy, constants.PresenceInvisible, constants.PresenceIdle:
	default:
		return errors.New("invalid presence")
	}
	now := time.Now().UTC()
	_ = s.Presence.Upsert(ctx, &models.UserPresence{UserID: userID, Status: status, Device: device, LastSeenAt: now})
	if u, err := s.Users.FindByID(ctx, userID); err == nil {
		u.LastSeenAt = &now
		_ = s.Users.Update(ctx, u)
	}
	_ = s.RDB.Set(ctx, constants.RedisPresenceKey+userID.String(), status, 2*time.Minute).Err()
	s.publish(ctx, constants.WSEventPresence, nil, &userID, map[string]interface{}{
		"userId": userID, "status": status, "lastSeenAt": now,
	})
	return nil
}

func (s *Services) PinMessage(ctx context.Context, userID, conversationID, messageID uuid.UUID, pin bool) error {
	if _, err := s.requireMember(ctx, conversationID, userID); err != nil {
		return err
	}
	msg, err := s.Messages.FindByID(ctx, messageID)
	if err != nil || msg.ConversationID != conversationID {
		return errors.New("message not found")
	}
	if pin {
		_ = s.Pins.Pin(ctx, &models.PinnedMessage{ConversationID: conversationID, MessageID: messageID, PinnedBy: userID})
		msg.IsPinned = true
	} else {
		_ = s.Pins.Unpin(ctx, conversationID, messageID)
		msg.IsPinned = false
	}
	_ = s.Messages.Update(ctx, msg)
	s.publish(ctx, constants.WSEventPin, &conversationID, &userID, map[string]interface{}{
		"messageId": messageID, "pinned": pin,
	})
	return nil
}

func (s *Services) ForwardMessages(ctx context.Context, userID uuid.UUID, targetConversationID uuid.UUID, messageIDs []uuid.UUID) ([]MessageView, error) {
	if _, err := s.requireMember(ctx, targetConversationID, userID); err != nil {
		return nil, err
	}
	out := make([]MessageView, 0, len(messageIDs))
	for _, mid := range messageIDs {
		src, err := s.Messages.FindByID(ctx, mid)
		if err != nil {
			continue
		}
		if _, err := s.requireMember(ctx, src.ConversationID, userID); err != nil {
			continue
		}
		fid := src.ID
		view, err := s.SendMessage(ctx, userID, targetConversationID, dto.SendMessageRequest{
			Type: src.Type, Body: src.Body, ClientMsgID: uuid.NewString(),
		})
		if err != nil {
			continue
		}
		view.Message.ForwardedFrom = &fid
		_ = s.Messages.Update(ctx, &view.Message)
		out = append(out, *view)
	}
	return out, nil
}

func (s *Services) Search(ctx context.Context, userID uuid.UUID, q, typ, conversationID string, limit int) (map[string]interface{}, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	result := map[string]interface{}{}
	var convID *uuid.UUID
	if conversationID != "" {
		id, err := uuid.Parse(conversationID)
		if err == nil {
			convID = &id
		}
	}
	msgs, err := s.Messages.Search(ctx, userID, q, convID, typ, limit)
	if err != nil {
		return nil, err
	}
	users, _ := s.Users.Search(ctx, q, limit)
	userDTOs := make([]dto.UserDTO, 0, len(users))
	for i := range users {
		userDTOs = append(userDTOs, toUserDTO(&users[i]))
	}
	result["messages"] = msgs
	result["users"] = userDTOs
	return result, nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func mustJSON(v interface{}) datatypes.JSON {
	b, err := json.Marshal(v)
	if err != nil {
		return datatypes.JSON([]byte("{}"))
	}
	return datatypes.JSON(b)
}
