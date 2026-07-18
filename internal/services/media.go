package services

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/pulse/chat-service/internal/constants"
	"github.com/pulse/chat-service/internal/dto"
	"github.com/pulse/chat-service/internal/models"
	"github.com/pulse/chat-service/internal/storage"
	"github.com/pulse/chat-service/internal/utils"
	"github.com/google/uuid"
)

func (s *Services) UploadAttachment(ctx context.Context, userID, conversationID uuid.UUID, fileName string, reader io.Reader, size int64, contentType string, body string, msgType string, durationMs int, waveform string) (*MessageView, error) {
	if _, err := s.requireMember(ctx, conversationID, userID); err != nil {
		return nil, err
	}
	saved, err := s.Store.Save(userID, fileName, reader, size, contentType)
	if err != nil {
		return nil, err
	}
	if err := storage.VirusScanHook(saved.StoragePath); err != nil {
		return nil, errors.New("file rejected by security scan")
	}
	if msgType == "" {
		msgType = inferMessageType(saved.MimeType)
	}
	view, err := s.SendMessage(ctx, userID, conversationID, dto.SendMessageRequest{
		Type: msgType, Body: utils.SanitizeText(body, 2000), ClientMsgID: uuid.NewString(),
	})
	if err != nil {
		return nil, err
	}
	att := &models.Attachment{
		MessageID: view.Message.ID, UploaderID: userID, FileName: saved.FileName,
		MimeType: saved.MimeType, SizeBytes: saved.SizeBytes, StoragePath: saved.StoragePath,
		URL: saved.URL, ThumbnailURL: saved.ThumbnailURL, Checksum: saved.Checksum, DurationMs: durationMs,
	}
	if err := s.Attachments.Create(ctx, att); err != nil {
		return nil, err
	}
	view.Attachments = []models.Attachment{*att}
	if msgType == constants.MessageVoice {
		v := &models.VoiceMessage{MessageID: view.Message.ID, DurationMs: durationMs, WaveformJSON: waveform, URL: saved.URL}
		_ = s.Voices.Create(ctx, v)
		view.Voice = v
		s.publish(ctx, constants.WSEventVoice, &conversationID, &userID, view)
	}
	// SendMessage already published WSEventMessage; republish once with attachments attached.
	s.publish(ctx, constants.WSEventMessage, &conversationID, &userID, view)
	return view, nil
}

func inferMessageType(mime string) string {
	switch {
	case len(mime) >= 5 && mime[:5] == "image":
		if mime == "image/gif" {
			return constants.MessageGIF
		}
		return constants.MessageImage
	case len(mime) >= 5 && mime[:5] == "video":
		return constants.MessageVideo
	case len(mime) >= 5 && mime[:5] == "audio":
		return constants.MessageAudio
	case mime == "application/pdf":
		return constants.MessagePDF
	case mime == "application/zip":
		return constants.MessageZIP
	case mime == "application/x-rar-compressed" || mime == "application/vnd.rar":
		return constants.MessageRAR
	case mime == "application/vnd.android.package-archive":
		return constants.MessageAPK
	default:
		return constants.MessageDocument
	}
}

func (s *Services) SaveDraft(ctx context.Context, userID, conversationID uuid.UUID, req dto.DraftRequest) (*models.DraftMessage, error) {
	if _, err := s.requireMember(ctx, conversationID, userID); err != nil {
		return nil, err
	}
	d := &models.DraftMessage{
		ConversationID: conversationID, UserID: userID,
		Body: utils.SanitizeText(req.Body, 10000), ReplyToID: req.ReplyToID,
	}
	if err := s.Drafts.Upsert(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

func (s *Services) GetDraft(ctx context.Context, userID, conversationID uuid.UUID) (*models.DraftMessage, error) {
	if _, err := s.requireMember(ctx, conversationID, userID); err != nil {
		return nil, err
	}
	return s.Drafts.Get(ctx, conversationID, userID)
}

func (s *Services) BlockUser(ctx context.Context, blocker uuid.UUID, req dto.BlockRequest) error {
	if blocker == req.UserID {
		return errors.New("cannot block yourself")
	}
	if _, err := s.Users.FindByID(ctx, req.UserID); err != nil {
		return errors.New("user not found")
	}
	return s.Blocks.Block(ctx, &models.BlockedUser{BlockerID: blocker, BlockedID: req.UserID, Reason: utils.SanitizeText(req.Reason, 255)})
}

func (s *Services) UnblockUser(ctx context.Context, blocker, blocked uuid.UUID) error {
	return s.Blocks.Unblock(ctx, blocker, blocked)
}

func (s *Services) ListBlocked(ctx context.Context, blocker uuid.UUID) ([]models.BlockedUser, error) {
	return s.Blocks.List(ctx, blocker)
}

func (s *Services) UpdateSettings(ctx context.Context, userID uuid.UUID, req dto.SettingsRequest) (*models.ChatSettings, error) {
	ssettings, err := s.Settings.GetOrCreate(ctx, userID)
	if err != nil {
		return nil, err
	}
	if req.Theme != nil {
		ssettings.Theme = *req.Theme
	}
	if req.NotificationEnabled != nil {
		ssettings.NotificationEnabled = *req.NotificationEnabled
	}
	if req.SoundEnabled != nil {
		ssettings.SoundEnabled = *req.SoundEnabled
	}
	if req.ReadReceiptsEnabled != nil {
		ssettings.ReadReceiptsEnabled = *req.ReadReceiptsEnabled
	}
	if req.LastSeenVisible != nil {
		ssettings.LastSeenVisible = *req.LastSeenVisible
	}
	if req.Language != nil {
		ssettings.Language = *req.Language
	}
	if err := s.Settings.Save(ctx, ssettings); err != nil {
		return nil, err
	}
	return ssettings, nil
}

func (s *Services) GetSettings(ctx context.Context, userID uuid.UUID) (*models.ChatSettings, error) {
	return s.Settings.GetOrCreate(ctx, userID)
}

func (s *Services) RegisterDevice(ctx context.Context, userID uuid.UUID, req dto.DeviceTokenRequest) error {
	return s.Devices.Upsert(ctx, &models.DeviceToken{
		UserID: userID, Token: utils.SanitizeText(req.Token, 512), Platform: utils.SanitizeText(req.Platform, 32), Active: true,
	})
}

func (s *Services) ListNotifications(ctx context.Context, userID uuid.UUID) ([]models.NotificationQueue, error) {
	return s.Notifications.List(ctx, userID, 50)
}

func (s *Services) StartCall(ctx context.Context, userID, conversationID uuid.UUID, callType string) (*models.CallHistory, error) {
	if _, err := s.requireMember(ctx, conversationID, userID); err != nil {
		return nil, err
	}
	if callType != "audio" && callType != "video" {
		return nil, errors.New("invalid call type")
	}
	now := time.Now().UTC()
	call := &models.CallHistory{
		ConversationID: conversationID, InitiatorID: userID, CallType: callType,
		Status: "ringing", StartedAt: &now,
	}
	if err := s.Calls.Create(ctx, call); err != nil {
		return nil, err
	}
	s.publish(ctx, constants.WSEventCall, &conversationID, &userID, call)
	return call, nil
}

func (s *Services) EndCall(ctx context.Context, userID, callID uuid.UUID) (*models.CallHistory, error) {
	var call models.CallHistory
	if err := s.DB.WithContext(ctx).First(&call, "id = ?", callID).Error; err != nil {
		return nil, err
	}
	if _, err := s.requireMember(ctx, call.ConversationID, userID); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	call.Status = "ended"
	call.EndedAt = &now
	if call.StartedAt != nil {
		call.DurationSec = int(now.Sub(*call.StartedAt).Seconds())
	}
	if err := s.Calls.Update(ctx, &call); err != nil {
		return nil, err
	}
	s.publish(ctx, constants.WSEventCall, &call.ConversationID, &userID, call)
	return &call, nil
}

func (s *Services) ListCalls(ctx context.Context, userID uuid.UUID) ([]models.CallHistory, error) {
	return s.Calls.List(ctx, userID, 50)
}

func (s *Services) StarMessage(ctx context.Context, userID, messageID uuid.UUID, star bool) error {
	msg, err := s.Messages.FindByID(ctx, messageID)
	if err != nil {
		return err
	}
	if _, err := s.requireMember(ctx, msg.ConversationID, userID); err != nil {
		return err
	}
	if star {
		return s.Stars.Star(ctx, &models.StarredMessage{MessageID: messageID, UserID: userID})
	}
	return s.Stars.Unstar(ctx, messageID, userID)
}

func (s *Services) ListStarred(ctx context.Context, userID uuid.UUID) ([]models.StarredMessage, error) {
	return s.Stars.List(ctx, userID, 100)
}

func (s *Services) ListPinned(ctx context.Context, userID, conversationID uuid.UUID) ([]models.PinnedMessage, error) {
	if _, err := s.requireMember(ctx, conversationID, userID); err != nil {
		return nil, err
	}
	return s.Pins.List(ctx, conversationID)
}
