package webrtc_test

import (
	"testing"

	"github.com/pulse/chat-service/internal/call/webrtc"
)

func TestValidateSDP(t *testing.T) {
	if !webrtc.ValidateSDP("v=0\r\no=- 0 0 IN IP4 127.0.0.1\r\nm=audio 9 UDP/TLS/RTP/SAVPF 111\r\n") {
		t.Fatal("expected valid sdp")
	}
	if webrtc.ValidateSDP("short") {
		t.Fatal("expected invalid")
	}
}
