package dto

import "github.com/google/uuid"

type StartCallRequest struct {
	CallType       string      `json:"callType" binding:"required"`
	ConversationID *uuid.UUID  `json:"conversationId"`
	ParticipantIDs []uuid.UUID `json:"participantIds" binding:"required,min=1"`
	IsGroup        bool        `json:"isGroup"`
}

type ICEServersResponse struct {
	ICEServers []ICEServer `json:"iceServers"`
}

type ICEServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

type QualityReport struct {
	LatencyMs      int     `json:"latencyMs"`
	JitterMs       int     `json:"jitterMs"`
	PacketLoss     float64 `json:"packetLoss"`
	BandwidthKbps  int     `json:"bandwidthKbps"`
	ConnectionType string  `json:"connectionType"`
	SignalStrength int     `json:"signalStrength"`
}

type SettingsRequest struct {
	DefaultCamera    *string `json:"defaultCamera"`
	NoiseSuppression *bool   `json:"noiseSuppression"`
	EchoCancellation *bool   `json:"echoCancellation"`
	AutoGainControl  *bool   `json:"autoGainControl"`
	MirrorLocalVideo *bool   `json:"mirrorLocalVideo"`
	MaxBitrateKbps   *int    `json:"maxBitrateKbps"`
}

type InviteRequest struct {
	UserIDs []uuid.UUID `json:"userIds" binding:"required,min=1"`
}

type SignalingMessage struct {
	Type   string      `json:"type"`
	CallID string      `json:"callId,omitempty"`
	To     string      `json:"to,omitempty"`
	From   string      `json:"from,omitempty"`
	Payload interface{} `json:"payload,omitempty"`
}
