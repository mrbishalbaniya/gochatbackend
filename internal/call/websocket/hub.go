package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pulse/chat-service/internal/call/constants"
	"github.com/pulse/chat-service/internal/call/events"
	"github.com/pulse/chat-service/internal/call/services"
	"github.com/pulse/chat-service/internal/utils"
	"github.com/redis/go-redis/v9"
)

type Client struct {
	ID     uuid.UUID
	UserID uuid.UUID
	Hub    *Hub
	Conn   *websocket.Conn
	Send   chan []byte
	Calls  map[uuid.UUID]struct{}
	mu     sync.Mutex
}

type Hub struct {
	Clients      map[uuid.UUID]*Client
	ByUser       map[uuid.UUID]map[uuid.UUID]*Client
	Register     chan *Client
	Unregister   chan *Client
	RDB          *redis.Client
	Svc          *services.Services
	AccessSecret string
	upgrader     websocket.Upgrader
	mu           sync.RWMutex
}

func NewHub(rdb *redis.Client, svc *services.Services, secret string, origins []string) *Hub {
	allowed := map[string]struct{}{}
	for _, o := range origins {
		allowed[o] = struct{}{}
	}
	return &Hub{
		Clients: make(map[uuid.UUID]*Client), ByUser: make(map[uuid.UUID]map[uuid.UUID]*Client),
		Register: make(chan *Client), Unregister: make(chan *Client),
		RDB: rdb, Svc: svc, AccessSecret: secret,
		upgrader: websocket.Upgrader{
			ReadBufferSize: 1024, WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				o := r.Header.Get("Origin")
				if o == "" {
					return true
				}
				_, ok := allowed[o]
				return ok
			},
		},
	}
}

func (h *Hub) Run(ctx context.Context) {
	go h.subscribe(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case c := <-h.Register:
			h.mu.Lock()
			h.Clients[c.ID] = c
			if h.ByUser[c.UserID] == nil {
				h.ByUser[c.UserID] = map[uuid.UUID]*Client{}
			}
			h.ByUser[c.UserID][c.ID] = c
			h.mu.Unlock()
			_ = h.RDB.Set(ctx, constants.RedisPresenceKey+c.UserID.String(), "online", 2*time.Minute).Err()
		case c := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[c.ID]; ok {
				delete(h.Clients, c.ID)
				delete(h.ByUser[c.UserID], c.ID)
				if len(h.ByUser[c.UserID]) == 0 {
					delete(h.ByUser, c.UserID)
					_ = h.RDB.Set(context.Background(), constants.RedisPresenceKey+c.UserID.String(), "offline", 10*time.Minute).Err()
				}
				close(c.Send)
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) Publish(ctx context.Context, evt events.Event) error {
	b, err := evt.Bytes()
	if err != nil {
		return err
	}
	ch := constants.RedisCallChannel + "global"
	if evt.CallID != nil {
		ch = constants.RedisCallChannel + evt.CallID.String()
	}
	return h.RDB.Publish(ctx, ch, b).Err()
}

func (h *Hub) subscribe(ctx context.Context) {
	pubsub := h.RDB.PSubscribe(ctx, constants.RedisCallChannel+"*")
	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			_ = pubsub.Close()
			return
		case msg := <-ch:
			if msg == nil {
				continue
			}
			h.dispatch([]byte(msg.Payload))
		}
	}
}

func (h *Hub) dispatch(raw []byte) {
	evt, err := events.Parse(raw)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Targeted user delivery for invites / 1:1 signaling with "to"
	var payload map[string]interface{}
	_ = json.Unmarshal(evt.Payload, &payload)
	if toStr, ok := payload["to"].(string); ok {
		if toID, err := uuid.Parse(toStr); err == nil {
			for _, c := range h.ByUser[toID] {
				select {
				case c.Send <- raw:
				default:
				}
			}
			return
		}
	}
	if evt.Type == constants.WSCallInvite && evt.CallID != nil {
		targets := inviteTargets(evt.Payload)
		if len(targets) == 0 {
			for _, c := range h.Clients {
				select {
				case c.Send <- raw:
				default:
				}
			}
			return
		}
		for uid := range targets {
			for _, c := range h.ByUser[uid] {
				c.mu.Lock()
				c.Calls[*evt.CallID] = struct{}{}
				c.mu.Unlock()
				select {
				case c.Send <- raw:
				default:
				}
			}
		}
		return
	}
	if evt.CallID != nil {
		for _, c := range h.Clients {
			c.mu.Lock()
			_, in := c.Calls[*evt.CallID]
			c.mu.Unlock()
			if in || evt.Type == constants.WSCallInvite || evt.Type == constants.WSCallEnd {
				select {
				case c.Send <- raw:
				default:
				}
			}
		}
	}
}

func (h *Hub) HandleWS(c *gin.Context) {
	token := c.Query("token")
	if token == "" && strings.HasPrefix(c.GetHeader("Authorization"), "Bearer ") {
		token = strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	}
	if token == "" {
		utils.Unauthorized(c, "missing token")
		return
	}
	claims, err := utils.ParseAccessToken(h.AccessSecret, token)
	if err != nil {
		utils.Unauthorized(c, "invalid token")
		return
	}
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("ws upgrade", "error", err)
		return
	}
	client := &Client{
		ID: uuid.New(), UserID: claims.UserID, Hub: h, Conn: conn,
		Send: make(chan []byte, 256), Calls: map[uuid.UUID]struct{}{},
	}
	h.Register <- client
	go client.writePump()
	go client.readPump()
}

type inbound struct {
	Type    string          `json:"type"`
	CallID  string          `json:"callId"`
	To      string          `json:"to"`
	Payload json.RawMessage `json:"payload"`
}

func (c *Client) readPump() {
	defer func() {
		c.Hub.Unregister <- c
		_ = c.Conn.Close()
	}()
	_ = c.Conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		_ = c.Conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})
	for {
		_, data, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}
		var in inbound
		if err := json.Unmarshal(data, &in); err != nil {
			continue
		}
		ctx := context.Background()
		switch in.Type {
		case constants.WSPing:
			pong, _ := events.New(constants.WSPong, nil, &c.UserID, map[string]string{"ok": "true"})
			b, _ := pong.Bytes()
			c.Send <- b
		case constants.WSJoinCall:
			callID, err := uuid.Parse(in.CallID)
			if err != nil {
				continue
			}
			// Host already joined; track for signaling. Invitee join == accept.
			if p, err := c.Hub.Svc.GetParticipant(ctx, callID, c.UserID); err == nil {
				if p.Status == constants.ParticipantRinging {
					_, _ = c.Hub.Svc.AcceptCall(ctx, c.UserID, callID)
				}
				c.mu.Lock()
				c.Calls[callID] = struct{}{}
				c.mu.Unlock()
			}
		case constants.WSAccept:
			callID, err := uuid.Parse(in.CallID)
			if err != nil {
				continue
			}
			if _, err := c.Hub.Svc.AcceptCall(ctx, c.UserID, callID); err == nil {
				c.mu.Lock()
				c.Calls[callID] = struct{}{}
				c.mu.Unlock()
			}
		case constants.WSReject:
			callID, err := uuid.Parse(in.CallID)
			if err != nil {
				continue
			}
			_ = c.Hub.Svc.RejectCall(ctx, c.UserID, callID)
		case constants.WSLeaveCall, constants.WSCallEnd:
			callID, err := uuid.Parse(in.CallID)
			if err != nil {
				continue
			}
			_, _ = c.Hub.Svc.EndCall(ctx, c.UserID, callID)
			c.mu.Lock()
			delete(c.Calls, callID)
			c.mu.Unlock()
		case constants.WSOffer:
			c.handleSDP(ctx, in, true)
		case constants.WSAnswer:
			c.handleSDP(ctx, in, false)
		case constants.WSIceCandidate:
			c.handleICE(ctx, in)
		case constants.WSMute, constants.WSUnmute, constants.WSCameraOn, constants.WSCameraOff,
			constants.WSScreenShareOn, constants.WSScreenShareOff, constants.WSRaiseHand:
			c.handleMedia(ctx, in)
		}
	}
}

func (c *Client) handleSDP(ctx context.Context, in inbound, offer bool) {
	callID, err := uuid.Parse(in.CallID)
	if err != nil {
		return
	}
	to, err := uuid.Parse(in.To)
	if err != nil {
		return
	}
	var p struct {
		SDP string `json:"sdp"`
	}
	_ = json.Unmarshal(in.Payload, &p)
	if offer {
		_ = c.Hub.Svc.SaveOffer(ctx, c.UserID, to, callID, p.SDP)
	} else {
		_ = c.Hub.Svc.SaveAnswer(ctx, c.UserID, to, callID, p.SDP)
	}
	c.mu.Lock()
	c.Calls[callID] = struct{}{}
	c.mu.Unlock()
}

func (c *Client) handleICE(ctx context.Context, in inbound) {
	callID, err := uuid.Parse(in.CallID)
	if err != nil {
		return
	}
	var p struct {
		Candidate     string `json:"candidate"`
		SDPMid        string `json:"sdpMid"`
		SDPMLineIndex int    `json:"sdpMLineIndex"`
	}
	_ = json.Unmarshal(in.Payload, &p)
	var to *uuid.UUID
	if in.To != "" {
		if id, err := uuid.Parse(in.To); err == nil {
			to = &id
		}
	}
	_ = c.Hub.Svc.SaveICE(ctx, c.UserID, callID, p.Candidate, p.SDPMid, p.SDPMLineIndex, to)
}

func (c *Client) handleMedia(ctx context.Context, in inbound) {
	callID, err := uuid.Parse(in.CallID)
	if err != nil {
		return
	}
	var muted, camera, screen, hand *bool
	t, f := true, false
	switch in.Type {
	case constants.WSMute:
		muted = &t
	case constants.WSUnmute:
		muted = &f
	case constants.WSCameraOn:
		camera = &t
	case constants.WSCameraOff:
		camera = &f
	case constants.WSScreenShareOn:
		screen = &t
	case constants.WSScreenShareOff:
		screen = &f
	case constants.WSRaiseHand:
		hand = &t
	}
	_, _ = c.Hub.Svc.UpdateMediaState(ctx, c.UserID, callID, muted, camera, screen, hand)
}

func inviteTargets(payload json.RawMessage) map[uuid.UUID]struct{} {
	out := map[uuid.UUID]struct{}{}
	var view struct {
		Participants []struct {
			UserID uuid.UUID `json:"userId"`
		} `json:"participants"`
		InviteeID string `json:"inviteeId"`
	}
	if err := json.Unmarshal(payload, &view); err != nil {
		return out
	}
	for _, p := range view.Participants {
		out[p.UserID] = struct{}{}
	}
	if view.InviteeID != "" {
		if id, err := uuid.Parse(view.InviteeID); err == nil {
			out[id] = struct{}{}
		}
	}
	return out
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		_ = c.Conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
