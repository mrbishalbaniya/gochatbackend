package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type User struct {
	Base
	Email        string     `gorm:"uniqueIndex;size:255;not null" json:"email"`
	Username     string     `gorm:"uniqueIndex;size:64;not null" json:"username"`
	DisplayName  string     `gorm:"size:120;not null" json:"displayName"`
	PasswordHash string     `gorm:"size:255;not null" json:"-"`
	AvatarURL    string     `gorm:"size:512" json:"avatarUrl"`
	Bio          string     `gorm:"size:500" json:"bio"`
	Role         string     `gorm:"size:32;default:user;not null" json:"role"`
	IsActive     bool       `gorm:"default:true;not null" json:"isActive"`
	LastSeenAt   *time.Time `json:"lastSeenAt"`
}

type RefreshToken struct {
	Base
	UserID    uuid.UUID `gorm:"type:uuid;index;not null" json:"userId"`
	TokenHash string    `gorm:"size:128;uniqueIndex;not null" json:"-"`
	ExpiresAt time.Time `gorm:"index;not null" json:"expiresAt"`
	RevokedAt *time.Time `json:"revokedAt"`
	UserAgent string    `gorm:"size:255" json:"userAgent"`
	IPAddress string    `gorm:"size:64" json:"ipAddress"`
}

type Conversation struct {
	Base
	Type        string         `gorm:"size:32;index;not null" json:"type"`
	Title       string         `gorm:"size:200" json:"title"`
	Description string         `gorm:"size:1000" json:"description"`
	AvatarURL   string         `gorm:"size:512" json:"avatarUrl"`
	CreatedBy   uuid.UUID      `gorm:"type:uuid;index;not null" json:"createdBy"`
	IsTemporary bool           `gorm:"default:false" json:"isTemporary"`
	ExpiresAt   *time.Time     `json:"expiresAt"`
	Metadata    datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata"`
	LastMessageAt *time.Time   `gorm:"index" json:"lastMessageAt"`
}

type ConversationMember struct {
	Base
	ConversationID uuid.UUID  `gorm:"type:uuid;uniqueIndex:idx_conv_user;not null" json:"conversationId"`
	UserID         uuid.UUID  `gorm:"type:uuid;uniqueIndex:idx_conv_user;index;not null" json:"userId"`
	Role           string     `gorm:"size:32;default:member;not null" json:"role"`
	Nickname       string     `gorm:"size:120" json:"nickname"`
	JoinedAt       time.Time  `gorm:"not null" json:"joinedAt"`
	LeftAt         *time.Time `json:"leftAt"`
	IsPinned       bool       `gorm:"default:false" json:"isPinned"`
	IsMuted        bool       `gorm:"default:false" json:"isMuted"`
	IsArchived     bool       `gorm:"default:false" json:"isArchived"`
	MuteUntil      *time.Time `json:"muteUntil"`
	LastReadAt     *time.Time `json:"lastReadAt"`
	LastReadMsgID  *uuid.UUID `gorm:"type:uuid" json:"lastReadMessageId"`
	UnreadCount    int        `gorm:"default:0" json:"unreadCount"`
}

type Message struct {
	Base
	ConversationID uuid.UUID      `gorm:"type:uuid;index:idx_msg_conv_created,priority:1;not null" json:"conversationId"`
	SenderID       uuid.UUID      `gorm:"type:uuid;index;not null" json:"senderId"`
	Type           string         `gorm:"size:32;not null" json:"type"`
	Body           string         `gorm:"type:text" json:"body"`
	ReplyToID      *uuid.UUID     `gorm:"type:uuid;index" json:"replyToId"`
	ForwardedFrom  *uuid.UUID     `gorm:"type:uuid" json:"forwardedFrom"`
	ClientMsgID    string         `gorm:"size:64;index" json:"clientMessageId"`
	Status         string         `gorm:"size:32;default:sent;not null" json:"status"`
	IsEdited       bool           `gorm:"default:false" json:"isEdited"`
	EditedAt       *time.Time     `json:"editedAt"`
	IsPinned       bool           `gorm:"default:false" json:"isPinned"`
	Metadata       datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata"`
	ScheduledAt    *time.Time     `json:"scheduledAt"`
}

type Attachment struct {
	Base
	MessageID    uuid.UUID `gorm:"type:uuid;index;not null" json:"messageId"`
	UploaderID   uuid.UUID `gorm:"type:uuid;index;not null" json:"uploaderId"`
	FileName     string    `gorm:"size:255;not null" json:"fileName"`
	MimeType     string    `gorm:"size:128;not null" json:"mimeType"`
	SizeBytes    int64     `gorm:"not null" json:"sizeBytes"`
	StoragePath  string    `gorm:"size:512;not null" json:"-"`
	URL          string    `gorm:"size:512;not null" json:"url"`
	ThumbnailURL string    `gorm:"size:512" json:"thumbnailUrl"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	DurationMs   int       `json:"durationMs"`
	Checksum     string    `gorm:"size:128" json:"checksum"`
}

type Reaction struct {
	Base
	MessageID uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_reaction_unique;not null" json:"messageId"`
	UserID    uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_reaction_unique;index;not null" json:"userId"`
	Emoji     string    `gorm:"size:32;uniqueIndex:idx_reaction_unique;not null" json:"emoji"`
}

type ReadReceipt struct {
	Base
	MessageID      uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_receipt_unique;not null" json:"messageId"`
	ConversationID uuid.UUID `gorm:"type:uuid;index;not null" json:"conversationId"`
	UserID         uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_receipt_unique;index;not null" json:"userId"`
	Status         string    `gorm:"size:32;not null" json:"status"`
	DeliveredAt    *time.Time `json:"deliveredAt"`
	ReadAt         *time.Time `json:"readAt"`
	SeenAt         *time.Time `json:"seenAt"`
}

type MessageStatus struct {
	Base
	MessageID uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_msg_status;not null" json:"messageId"`
	UserID    uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_msg_status;index;not null" json:"userId"`
	Status    string    `gorm:"size:32;not null" json:"status"`
}

type TypingStatus struct {
	Base
	ConversationID uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_typing;not null" json:"conversationId"`
	UserID         uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_typing;not null" json:"userId"`
	IsTyping       bool      `gorm:"default:false" json:"isTyping"`
	IsRecording    bool      `gorm:"default:false" json:"isRecording"`
	ExpiresAt      time.Time `gorm:"index;not null" json:"expiresAt"`
}

type PinnedMessage struct {
	Base
	ConversationID uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_pin_msg;not null" json:"conversationId"`
	MessageID      uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_pin_msg;not null" json:"messageId"`
	PinnedBy       uuid.UUID `gorm:"type:uuid;not null" json:"pinnedBy"`
}

type MessageEdit struct {
	Base
	MessageID uuid.UUID `gorm:"type:uuid;index;not null" json:"messageId"`
	EditorID  uuid.UUID `gorm:"type:uuid;not null" json:"editorId"`
	OldBody   string    `gorm:"type:text" json:"oldBody"`
	NewBody   string    `gorm:"type:text" json:"newBody"`
}

type VoiceMessage struct {
	Base
	MessageID    uuid.UUID `gorm:"type:uuid;uniqueIndex;not null" json:"messageId"`
	DurationMs   int       `gorm:"not null" json:"durationMs"`
	WaveformJSON string    `gorm:"type:text" json:"waveformJson"`
	URL          string    `gorm:"size:512;not null" json:"url"`
}

type DraftMessage struct {
	Base
	ConversationID uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_draft;not null" json:"conversationId"`
	UserID         uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_draft;not null" json:"userId"`
	Body           string    `gorm:"type:text" json:"body"`
	ReplyToID      *uuid.UUID `gorm:"type:uuid" json:"replyToId"`
}

type Mention struct {
	Base
	MessageID      uuid.UUID `gorm:"type:uuid;index;not null" json:"messageId"`
	ConversationID uuid.UUID `gorm:"type:uuid;index;not null" json:"conversationId"`
	MentionedUser  uuid.UUID `gorm:"type:uuid;index;not null" json:"mentionedUserId"`
	MentionedBy    uuid.UUID `gorm:"type:uuid;not null" json:"mentionedBy"`
}

type DeletedMessage struct {
	Base
	MessageID uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_deleted_msg;not null" json:"messageId"`
	UserID    uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_deleted_msg;index;not null" json:"userId"`
	Scope     string    `gorm:"size:32;not null" json:"scope"` // me | everyone
}

type ArchivedChat struct {
	Base
	ConversationID uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_archived;not null" json:"conversationId"`
	UserID         uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_archived;not null" json:"userId"`
}

type MutedChat struct {
	Base
	ConversationID uuid.UUID  `gorm:"type:uuid;uniqueIndex:idx_muted;not null" json:"conversationId"`
	UserID         uuid.UUID  `gorm:"type:uuid;uniqueIndex:idx_muted;not null" json:"userId"`
	MuteUntil      *time.Time `json:"muteUntil"`
}

type BlockedUser struct {
	Base
	BlockerID uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_block;not null" json:"blockerId"`
	BlockedID uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_block;index;not null" json:"blockedId"`
	Reason    string    `gorm:"size:255" json:"reason"`
}

type NotificationQueue struct {
	Base
	UserID  uuid.UUID      `gorm:"type:uuid;index;not null" json:"userId"`
	Type    string         `gorm:"size:64;not null" json:"type"`
	Title   string         `gorm:"size:200;not null" json:"title"`
	Body    string         `gorm:"size:1000" json:"body"`
	Payload datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"payload"`
	Status  string         `gorm:"size:32;default:pending;index;not null" json:"status"`
	SentAt  *time.Time     `json:"sentAt"`
}

type CallHistory struct {
	Base
	ConversationID uuid.UUID  `gorm:"type:uuid;index;not null" json:"conversationId"`
	InitiatorID    uuid.UUID  `gorm:"type:uuid;index;not null" json:"initiatorId"`
	CallType       string     `gorm:"size:32;not null" json:"callType"` // audio | video
	Status         string     `gorm:"size:32;not null" json:"status"`
	StartedAt      *time.Time `json:"startedAt"`
	EndedAt        *time.Time `json:"endedAt"`
	DurationSec    int        `json:"durationSec"`
}

type UserPresence struct {
	Base
	UserID    uuid.UUID `gorm:"type:uuid;uniqueIndex;not null" json:"userId"`
	Status    string    `gorm:"size:32;not null" json:"status"`
	Device    string    `gorm:"size:64" json:"device"`
	LastSeenAt time.Time `gorm:"index;not null" json:"lastSeenAt"`
}

type DeviceToken struct {
	Base
	UserID   uuid.UUID `gorm:"type:uuid;index;not null" json:"userId"`
	Token    string    `gorm:"size:512;uniqueIndex;not null" json:"token"`
	Platform string    `gorm:"size:32;not null" json:"platform"`
	Active   bool      `gorm:"default:true" json:"active"`
}

type ChatSettings struct {
	Base
	UserID              uuid.UUID `gorm:"type:uuid;uniqueIndex;not null" json:"userId"`
	Theme               string    `gorm:"size:32;default:system" json:"theme"`
	NotificationEnabled bool      `gorm:"default:true" json:"notificationEnabled"`
	SoundEnabled        bool      `gorm:"default:true" json:"soundEnabled"`
	ReadReceiptsEnabled bool      `gorm:"default:true" json:"readReceiptsEnabled"`
	LastSeenVisible     bool      `gorm:"default:true" json:"lastSeenVisible"`
	Language            string    `gorm:"size:16;default:en" json:"language"`
}

type StarredMessage struct {
	Base
	MessageID uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_star;not null" json:"messageId"`
	UserID    uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_star;index;not null" json:"userId"`
}
