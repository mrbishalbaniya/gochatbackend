package dto

import (
	"time"

	"github.com/google/uuid"
)

type RegisterRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Username    string `json:"username" binding:"required,min=3,max=32"`
	DisplayName string `json:"displayName" binding:"required,min=1,max=120"`
	Password    string `json:"password" binding:"required,min=8,max=128"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type AuthResponse struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
	User         UserDTO   `json:"user"`
}

type UserDTO struct {
	ID          uuid.UUID  `json:"id"`
	Email       string     `json:"email"`
	Username    string     `json:"username"`
	DisplayName string     `json:"displayName"`
	AvatarURL   string     `json:"avatarUrl"`
	Bio         string     `json:"bio"`
	Role        string     `json:"role"`
	LastSeenAt  *time.Time `json:"lastSeenAt"`
	Presence    string     `json:"presence,omitempty"`
}

type UpdateProfileRequest struct {
	DisplayName string `json:"displayName"`
	Bio         string `json:"bio"`
	AvatarURL   string `json:"avatarUrl"`
}

type CreateConversationRequest struct {
	Type        string      `json:"type" binding:"required"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	MemberIDs   []uuid.UUID `json:"memberIds"`
	IsTemporary bool        `json:"isTemporary"`
}

type SendMessageRequest struct {
	Type          string     `json:"type" binding:"required"`
	Body          string     `json:"body"`
	ReplyToID     *uuid.UUID `json:"replyToId"`
	ClientMsgID   string     `json:"clientMessageId"`
	Metadata      map[string]interface{} `json:"metadata"`
	ScheduledAt   *time.Time `json:"scheduledAt"`
}

type EditMessageRequest struct {
	Body string `json:"body" binding:"required"`
}

type ReactionRequest struct {
	Emoji string `json:"emoji" binding:"required,min=1,max=32"`
}

type ReceiptRequest struct {
	MessageIDs []uuid.UUID `json:"messageIds" binding:"required"`
	Status     string      `json:"status" binding:"required"`
}

type TypingRequest struct {
	IsTyping    bool `json:"isTyping"`
	IsRecording bool `json:"isRecording"`
}

type PresenceRequest struct {
	Status string `json:"status" binding:"required"`
}

type MuteRequest struct {
	MuteUntil *time.Time `json:"muteUntil"`
}

type BlockRequest struct {
	UserID uuid.UUID `json:"userId" binding:"required"`
	Reason string    `json:"reason"`
}

type DraftRequest struct {
	Body      string     `json:"body"`
	ReplyToID *uuid.UUID `json:"replyToId"`
}

type SearchQuery struct {
	Q              string `form:"q" binding:"required,min=1"`
	ConversationID string `form:"conversationId"`
	Type           string `form:"type"`
	Limit          int    `form:"limit"`
}

type SettingsRequest struct {
	Theme               *string `json:"theme"`
	NotificationEnabled *bool   `json:"notificationEnabled"`
	SoundEnabled        *bool   `json:"soundEnabled"`
	ReadReceiptsEnabled *bool   `json:"readReceiptsEnabled"`
	LastSeenVisible     *bool   `json:"lastSeenVisible"`
	Language            *string `json:"language"`
}

type DeviceTokenRequest struct {
	Token    string `json:"token" binding:"required"`
	Platform string `json:"platform" binding:"required"`
	P256dh   string `json:"p256dh"`
	Auth     string `json:"auth"`
}

type PushSubscribeRequest struct {
	Endpoint string `json:"endpoint" binding:"required"`
	Keys     struct {
		P256dh string `json:"p256dh" binding:"required"`
		Auth   string `json:"auth" binding:"required"`
	} `json:"keys" binding:"required"`
}

type CallRequest struct {
	CallType string `json:"callType" binding:"required"`
}

type PaginationMeta struct {
	NextCursor string `json:"nextCursor,omitempty"`
	HasMore    bool   `json:"hasMore"`
	Limit      int    `json:"limit"`
}
