package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Event struct {
	Type      string          `json:"type"`
	CallID    *uuid.UUID      `json:"callId,omitempty"`
	UserID    *uuid.UUID      `json:"userId,omitempty"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
}

func New(typ string, callID, userID *uuid.UUID, payload interface{}) (Event, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Event{}, err
	}
	return Event{Type: typ, CallID: callID, UserID: userID, Payload: raw, Timestamp: time.Now().UTC()}, nil
}

func (e Event) Bytes() ([]byte, error) { return json.Marshal(e) }

func Parse(b []byte) (Event, error) {
	var e Event
	err := json.Unmarshal(b, &e)
	return e, err
}
