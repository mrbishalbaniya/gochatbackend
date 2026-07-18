package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Base struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (b *Base) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}

// User is a lightweight projection; identity is owned by chat-service JWT claims.
type User struct {
	Base
	ExternalID  uuid.UUID `gorm:"type:uuid;uniqueIndex;not null" json:"externalId"`
	DisplayName string    `gorm:"size:120" json:"displayName"`
	AvatarURL   string    `gorm:"size:512" json:"avatarUrl"`
}

func (User) TableName() string { return "pulse_call_users" }

type Call struct {
	Base
	ConversationID  *uuid.UUID     `gorm:"type:uuid;index" json:"conversationId"`
	InitiatorID     uuid.UUID      `gorm:"type:uuid;index;not null" json:"initiatorId"`
	CallType        string         `gorm:"size:16;not null;index" json:"callType"`
	Status          string         `gorm:"size:32;not null;index" json:"status"`
	StartedAt       *time.Time     `json:"startedAt"`
	EndedAt         *time.Time     `json:"endedAt"`
	DurationSec     int            `json:"durationSec"`
	MaxParticipants int            `gorm:"default:12" json:"maxParticipants"`
	IsGroup         bool           `gorm:"default:false" json:"isGroup"`
	RecordingURL    string         `gorm:"size:512" json:"recordingUrl"`
	Metadata        datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata"`
}

func (Call) TableName() string { return "pulse_calls" }

type Participant struct {
	Base
	CallID      uuid.UUID  `gorm:"type:uuid;uniqueIndex:idx_call_user;not null" json:"callId"`
	UserID      uuid.UUID  `gorm:"type:uuid;uniqueIndex:idx_call_user;index;not null" json:"userId"`
	Role        string     `gorm:"size:32;default:participant" json:"role"`
	Status      string     `gorm:"size:32;not null" json:"status"`
	Muted       bool       `gorm:"default:false" json:"muted"`
	CameraOn    bool       `gorm:"default:false" json:"cameraOn"`
	ScreenShare bool       `gorm:"default:false" json:"screenShare"`
	RaisedHand  bool       `gorm:"default:false" json:"raisedHand"`
	JoinedAt    *time.Time `json:"joinedAt"`
	LeftAt      *time.Time `json:"leftAt"`
	DeviceID    string     `gorm:"size:128" json:"deviceId"`
}

func (Participant) TableName() string { return "pulse_call_participants" }

type ICECandidate struct {
	Base
	CallID        uuid.UUID `gorm:"type:uuid;index;not null" json:"callId"`
	UserID        uuid.UUID `gorm:"type:uuid;index;not null" json:"userId"`
	Candidate     string    `gorm:"type:text;not null" json:"candidate"`
	SDPMid        string    `gorm:"size:64" json:"sdpMid"`
	SDPMLineIndex int       `json:"sdpMLineIndex"`
}

func (ICECandidate) TableName() string { return "pulse_call_ice_candidates" }

type CallOffer struct {
	Base
	CallID   uuid.UUID `gorm:"type:uuid;index;not null" json:"callId"`
	FromUser uuid.UUID `gorm:"type:uuid;index;not null" json:"fromUser"`
	ToUser   uuid.UUID `gorm:"type:uuid;index;not null" json:"toUser"`
	SDP      string    `gorm:"type:text;not null" json:"sdp"`
}

func (CallOffer) TableName() string { return "pulse_call_offers" }

type CallAnswer struct {
	Base
	CallID   uuid.UUID `gorm:"type:uuid;index;not null" json:"callId"`
	FromUser uuid.UUID `gorm:"type:uuid;index;not null" json:"fromUser"`
	ToUser   uuid.UUID `gorm:"type:uuid;index;not null" json:"toUser"`
	SDP      string    `gorm:"type:text;not null" json:"sdp"`
}

func (CallAnswer) TableName() string { return "pulse_call_answers" }

type CallLog struct {
	Base
	CallID  uuid.UUID      `gorm:"type:uuid;index;not null" json:"callId"`
	UserID  *uuid.UUID     `gorm:"type:uuid;index" json:"userId"`
	Event   string         `gorm:"size:64;not null" json:"event"`
	Payload datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"payload"`
}

func (CallLog) TableName() string { return "pulse_call_logs" }

type CallInvitation struct {
	Base
	CallID    uuid.UUID  `gorm:"type:uuid;index;not null" json:"callId"`
	InviterID uuid.UUID  `gorm:"type:uuid;not null" json:"inviterId"`
	InviteeID uuid.UUID  `gorm:"type:uuid;index;not null" json:"inviteeId"`
	Status    string     `gorm:"size:32;default:pending" json:"status"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

func (CallInvitation) TableName() string { return "pulse_call_invitations" }

type CallRecording struct {
	Base
	CallID      uuid.UUID `gorm:"type:uuid;index;not null" json:"callId"`
	StartedBy   uuid.UUID `gorm:"type:uuid;not null" json:"startedBy"`
	StorageURL  string    `gorm:"size:512" json:"storageUrl"`
	DurationSec int       `json:"durationSec"`
	Status      string    `gorm:"size:32;default:pending" json:"status"`
}

func (CallRecording) TableName() string { return "pulse_call_recordings" }

type MissedCall struct {
	Base
	CallID   uuid.UUID `gorm:"type:uuid;index;not null" json:"callId"`
	UserID   uuid.UUID `gorm:"type:uuid;index;not null" json:"userId"`
	FromUser uuid.UUID `gorm:"type:uuid;not null" json:"fromUser"`
	Seen     bool      `gorm:"default:false" json:"seen"`
}

func (MissedCall) TableName() string { return "pulse_call_missed" }

type BlockedUser struct {
	Base
	BlockerID uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_call_block;not null" json:"blockerId"`
	BlockedID uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_call_block;index;not null" json:"blockedId"`
}

func (BlockedUser) TableName() string { return "pulse_call_blocked_users" }

type CallDevice struct {
	Base
	UserID     uuid.UUID `gorm:"type:uuid;index;not null" json:"userId"`
	DeviceID   string    `gorm:"size:128;uniqueIndex;not null" json:"deviceId"`
	Platform   string    `gorm:"size:32" json:"platform"`
	PushToken  string    `gorm:"size:512" json:"pushToken"`
	LastSeenAt time.Time `json:"lastSeenAt"`
}

func (CallDevice) TableName() string { return "pulse_call_devices" }

type CallSettings struct {
	Base
	UserID           uuid.UUID `gorm:"type:uuid;uniqueIndex;not null" json:"userId"`
	DefaultCamera    string    `gorm:"size:16;default:user" json:"defaultCamera"`
	NoiseSuppression bool      `gorm:"default:true" json:"noiseSuppression"`
	EchoCancellation bool      `gorm:"default:true" json:"echoCancellation"`
	AutoGainControl  bool      `gorm:"default:true" json:"autoGainControl"`
	MirrorLocalVideo bool      `gorm:"default:true" json:"mirrorLocalVideo"`
	MaxBitrateKbps   int       `gorm:"default:1500" json:"maxBitrateKbps"`
}

func (CallSettings) TableName() string { return "pulse_call_settings" }

type CallQuality struct {
	Base
	CallID         uuid.UUID `gorm:"type:uuid;index;not null" json:"callId"`
	UserID         uuid.UUID `gorm:"type:uuid;index;not null" json:"userId"`
	LatencyMs      int       `json:"latencyMs"`
	JitterMs       int       `json:"jitterMs"`
	PacketLoss     float64   `json:"packetLoss"`
	BandwidthKbps  int       `json:"bandwidthKbps"`
	ConnectionType string    `gorm:"size:32" json:"connectionType"`
	SignalStrength int       `json:"signalStrength"`
}

func (CallQuality) TableName() string { return "pulse_call_quality" }

type CallEvent struct {
	Base
	CallID  uuid.UUID      `gorm:"type:uuid;index;not null" json:"callId"`
	UserID  *uuid.UUID     `gorm:"type:uuid;index" json:"userId"`
	Type    string         `gorm:"size:64;not null;index" json:"type"`
	Payload datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"payload"`
}

func (CallEvent) TableName() string { return "pulse_call_events" }

type ScreenSharing struct {
	Base
	CallID    uuid.UUID  `gorm:"type:uuid;index;not null" json:"callId"`
	UserID    uuid.UUID  `gorm:"type:uuid;index;not null" json:"userId"`
	ShareType string     `gorm:"size:32;default:screen" json:"shareType"`
	StartedAt time.Time  `json:"startedAt"`
	EndedAt   *time.Time `json:"endedAt"`
	Active    bool       `gorm:"default:true" json:"active"`
}

func (ScreenSharing) TableName() string { return "pulse_call_screen_sharing" }

type CallNotification struct {
	Base
	UserID  uuid.UUID      `gorm:"type:uuid;index;not null" json:"userId"`
	CallID  uuid.UUID      `gorm:"type:uuid;index;not null" json:"callId"`
	Type    string         `gorm:"size:64;not null" json:"type"`
	Title   string         `gorm:"size:200" json:"title"`
	Body    string         `gorm:"size:500" json:"body"`
	Payload datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"payload"`
	Status  string         `gorm:"size:32;default:pending" json:"status"`
	SentAt  *time.Time     `json:"sentAt"`
}

func (CallNotification) TableName() string { return "pulse_call_notifications" }
