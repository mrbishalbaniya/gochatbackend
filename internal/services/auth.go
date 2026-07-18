package services

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/pulse/chat-service/internal/config"
	"github.com/pulse/chat-service/internal/constants"
	"github.com/pulse/chat-service/internal/dto"
	"github.com/pulse/chat-service/internal/events"
	"github.com/pulse/chat-service/internal/models"
	"github.com/pulse/chat-service/internal/repositories"
	"github.com/pulse/chat-service/internal/storage"
	"github.com/pulse/chat-service/internal/utils"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type EventPublisher interface {
	Publish(ctx context.Context, evt events.Event) error
}

type Services struct {
	Cfg       *config.Config
	DB        *gorm.DB
	RDB       *redis.Client
	Store     *storage.LocalStorage
	Publisher EventPublisher

	Users         *repositories.UserRepo
	RefreshTokens *repositories.RefreshRepo
	Conversations *repositories.ConversationRepo
	Messages      *repositories.MessageRepo
	Attachments   *repositories.AttachmentRepo
	Reactions     *repositories.ReactionRepo
	Receipts      *repositories.ReceiptRepo
	Pins          *repositories.PinRepo
	Blocks        *repositories.BlockRepo
	Presence      *repositories.PresenceRepo
	Drafts        *repositories.DraftRepo
	Settings      *repositories.SettingsRepo
	Notifications *repositories.NotificationRepo
	Calls         *repositories.CallRepo
	Devices       *repositories.DeviceRepo
	Voices        *repositories.VoiceRepo
	Deleted       *repositories.DeletedRepo
	Edits         *repositories.EditRepo
	Mentions      *repositories.MentionRepo
	Stars         *repositories.StarRepo
	Typing        *repositories.TypingRepo
}

func New(cfg *config.Config, db *gorm.DB, rdb *redis.Client, store *storage.LocalStorage, pub EventPublisher) *Services {
	return &Services{
		Cfg: cfg, DB: db, RDB: rdb, Store: store, Publisher: pub,
		Users: &repositories.UserRepo{DB: db}, RefreshTokens: &repositories.RefreshRepo{DB: db},
		Conversations: &repositories.ConversationRepo{DB: db}, Messages: &repositories.MessageRepo{DB: db},
		Attachments: &repositories.AttachmentRepo{DB: db}, Reactions: &repositories.ReactionRepo{DB: db},
		Receipts: &repositories.ReceiptRepo{DB: db}, Pins: &repositories.PinRepo{DB: db},
		Blocks: &repositories.BlockRepo{DB: db}, Presence: &repositories.PresenceRepo{DB: db},
		Drafts: &repositories.DraftRepo{DB: db}, Settings: &repositories.SettingsRepo{DB: db},
		Notifications: &repositories.NotificationRepo{DB: db}, Calls: &repositories.CallRepo{DB: db},
		Devices: &repositories.DeviceRepo{DB: db}, Voices: &repositories.VoiceRepo{DB: db},
		Deleted: &repositories.DeletedRepo{DB: db}, Edits: &repositories.EditRepo{DB: db},
		Mentions: &repositories.MentionRepo{DB: db}, Stars: &repositories.StarRepo{DB: db},
		Typing: &repositories.TypingRepo{DB: db},
	}
}

func toUserDTO(u *models.User) dto.UserDTO {
	return dto.UserDTO{
		ID: u.ID, Email: u.Email, Username: u.Username, DisplayName: u.DisplayName,
		AvatarURL: u.AvatarURL, Bio: u.Bio, Role: u.Role, LastSeenAt: u.LastSeenAt,
	}
}

func (s *Services) Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error) {
	email := utils.SanitizeText(req.Email, 255)
	username := utils.SanitizeText(req.Username, 32)
	if !utils.IsValidEmail(email) {
		return nil, errors.New("invalid email")
	}
	if !utils.IsValidUsername(username) {
		return nil, errors.New("invalid username")
	}
	if _, err := s.Users.FindByEmail(ctx, email); err == nil {
		return nil, errors.New("email already registered")
	}
	if _, err := s.Users.FindByUsername(ctx, username); err == nil {
		return nil, errors.New("username already taken")
	}
	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	user := &models.User{
		Email: email, Username: username, DisplayName: utils.SanitizeText(req.DisplayName, 120),
		PasswordHash: hash, Role: "user", IsActive: true,
	}
	if err := s.Users.Create(ctx, user); err != nil {
		return nil, err
	}
	if _, err := s.Settings.GetOrCreate(ctx, user.ID); err != nil {
		return nil, err
	}
	_ = s.Presence.Upsert(ctx, &models.UserPresence{UserID: user.ID, Status: constants.PresenceOffline, LastSeenAt: time.Now().UTC()})
	return s.issueAuth(ctx, user, "", "")
}

func (s *Services) Login(ctx context.Context, req dto.LoginRequest, ua, ip string) (*dto.AuthResponse, error) {
	user, err := s.Users.FindByEmail(ctx, req.Email)
	if err != nil || !utils.CheckPassword(user.PasswordHash, req.Password) {
		return nil, errors.New("invalid credentials")
	}
	if !user.IsActive {
		return nil, errors.New("account disabled")
	}
	return s.issueAuth(ctx, user, ua, ip)
}

func (s *Services) Refresh(ctx context.Context, refreshToken, ua, ip string) (*dto.AuthResponse, error) {
	hash := utils.HashToken(refreshToken)
	rt, err := s.RefreshTokens.FindValid(ctx, hash)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}
	user, err := s.Users.FindByID(ctx, rt.UserID)
	if err != nil {
		return nil, err
	}
	_ = s.RefreshTokens.Revoke(ctx, rt.ID)
	return s.issueAuth(ctx, user, ua, ip)
}

func (s *Services) Logout(ctx context.Context, refreshToken string) error {
	hash := utils.HashToken(refreshToken)
	rt, err := s.RefreshTokens.FindValid(ctx, hash)
	if err != nil {
		return nil
	}
	return s.RefreshTokens.Revoke(ctx, rt.ID)
}

func (s *Services) issueAuth(ctx context.Context, user *models.User, ua, ip string) (*dto.AuthResponse, error) {
	access, exp, err := utils.IssueAccessToken(s.Cfg.JWTAccessSecret, s.Cfg.AccessTokenTTL, user.ID, user.Email, user.Username, user.Role)
	if err != nil {
		return nil, err
	}
	raw, err := utils.RandomToken(32)
	if err != nil {
		return nil, err
	}
	rt := &models.RefreshToken{
		UserID: user.ID, TokenHash: utils.HashToken(raw),
		ExpiresAt: time.Now().UTC().Add(s.Cfg.RefreshTokenTTL), UserAgent: ua, IPAddress: ip,
	}
	if err := s.RefreshTokens.Create(ctx, rt); err != nil {
		return nil, err
	}
	return &dto.AuthResponse{
		AccessToken: access, RefreshToken: raw, ExpiresAt: exp, User: toUserDTO(user),
	}, nil
}

func (s *Services) GetProfile(ctx context.Context, userID uuid.UUID) (*dto.UserDTO, error) {
	u, err := s.Users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	d := toUserDTO(u)
	if p, err := s.Presence.Get(ctx, userID); err == nil {
		d.Presence = p.Status
	}
	return &d, nil
}

func (s *Services) UpdateProfile(ctx context.Context, userID uuid.UUID, req dto.UpdateProfileRequest) (*dto.UserDTO, error) {
	u, err := s.Users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if req.DisplayName != "" {
		u.DisplayName = utils.SanitizeText(req.DisplayName, 120)
	}
	if req.Bio != "" {
		u.Bio = utils.SanitizeText(req.Bio, 500)
	}
	if req.AvatarURL != "" {
		u.AvatarURL = utils.SanitizeText(req.AvatarURL, 512)
	}
	if err := s.Users.Update(ctx, u); err != nil {
		return nil, err
	}
	d := toUserDTO(u)
	return &d, nil
}

func (s *Services) publish(ctx context.Context, typ string, convID, userID *uuid.UUID, payload interface{}) {
	if s.Publisher == nil {
		return
	}
	evt, err := events.New(typ, convID, userID, payload)
	if err != nil {
		return
	}
	_ = s.Publisher.Publish(ctx, evt)
}

func jsonMeta(m map[string]interface{}) datatypes.JSON {
	if m == nil {
		return datatypes.JSON([]byte("{}"))
	}
	b, err := json.Marshal(m)
	if err != nil {
		return datatypes.JSON([]byte("{}"))
	}
	return datatypes.JSON(b)
}
