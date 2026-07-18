package turn

import (
	"github.com/pulse/chat-service/internal/config"
	"github.com/pulse/chat-service/internal/call/dto"
)

// ICEServers builds browser RTCIceServer config from self-hosted STUN/TURN.
func ICEServers(cfg *config.Config) dto.ICEServersResponse {
	servers := make([]dto.ICEServer, 0, 2)
	if len(cfg.STUNURLs) > 0 {
		servers = append(servers, dto.ICEServer{URLs: cfg.STUNURLs})
	}
	if len(cfg.TURNURLs) > 0 {
		servers = append(servers, dto.ICEServer{
			URLs:       cfg.TURNURLs,
			Username:   cfg.TURNUsername,
			Credential: cfg.TURNCredential,
		})
	}
	return dto.ICEServersResponse{ICEServers: servers}
}
