package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Event struct {
	Type           string          `json:"type"`
	ConversationID *uuid.UUID      `json:"conversationId,omitempty"`
	UserID         *uuid.UUID      `json:"userId,omitempty"`
	Payload        json.RawMessage `json:"payload"`
	Timestamp      time.Time       `json:"timestamp"`
}

func New(eventType string, conversationID, userID *uuid.UUID, payload interface{}) (Event, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Event{}, err
	}
	return Event{
		Type:           eventType,
		ConversationID: conversationID,
		UserID:         userID,
		Payload:        raw,
		Timestamp:      time.Now().UTC(),
	}, nil
}

func (e Event) Bytes() ([]byte, error) {
	return json.Marshal(e)
}

func Parse(data []byte) (Event, error) {
	var e Event
	err := json.Unmarshal(data, &e)
	return e, err
}
