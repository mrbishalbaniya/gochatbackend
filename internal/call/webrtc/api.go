package webrtc

import (
	"github.com/pion/webrtc/v4"
)

// NewAPI creates a Pion API with default codecs for SFU/peer helpers.
func NewAPI() (*webrtc.API, error) {
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		return nil, err
	}
	s := webrtc.SettingEngine{}
	return webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithSettingEngine(s)), nil
}

// ValidateSDP performs a lightweight SDP presence check (full parse happens in browsers / Pion peers).
func ValidateSDP(sdp string) bool {
	return len(sdp) > 32 && (contains(sdp, "v=0") || contains(sdp, "m=audio") || contains(sdp, "m=video"))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		})())
}
