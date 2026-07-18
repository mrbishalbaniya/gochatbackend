package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pulse/chat-service/internal/constants"
	"github.com/pulse/chat-service/internal/events"
	"github.com/pulse/chat-service/internal/services"
	"github.com/pulse/chat-service/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

func newUpgrader(allowedOrigins []string) websocket.Upgrader {
	allowed := map[string]struct{}{}
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			_, ok := allowed[origin]
			return ok
		},
	}
}

type Client struct {
	ID     uuid.UUID
	UserID uuid.UUID
	Hub    *Hub
	Conn   *websocket.Conn
	Send   chan []byte
	Rooms  map[uuid.UUID]struct{}
	mu     sync.Mutex
}

type Hub struct {
	Clients      map[uuid.UUID]*Client
	UserIndex    map[uuid.UUID]map[uuid.UUID]*Client
	Register     chan *Client
	Unregister   chan *Client
	Broadcast    chan []byte
	RDB          *redis.Client
	Svc          *services.Services
	AccessSecret string
	CORSOrigins  []string
	upgrader     websocket.Upgrader
	mu           sync.RWMutex
}

func NewHub(rdb *redis.Client, svc *services.Services, accessSecret string, corsOrigins []string) *Hub {
	return &Hub{
		Clients:      make(map[uuid.UUID]*Client),
		UserIndex:    make(map[uuid.UUID]map[uuid.UUID]*Client),
		Register:     make(chan *Client),
		Unregister:   make(chan *Client),
		Broadcast:    make(chan []byte, 256),
		RDB:          rdb,
		Svc:          svc,
		AccessSecret: accessSecret,
		CORSOrigins:  corsOrigins,
		upgrader:     newUpgrader(corsOrigins),
	}
}

func (h *Hub) Run(ctx context.Context) {
	go h.subscribeRedis(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case client := <-h.Register:
			h.mu.Lock()
			h.Clients[client.ID] = client
			if h.UserIndex[client.UserID] == nil {
				h.UserIndex[client.UserID] = map[uuid.UUID]*Client{}
			}
			h.UserIndex[client.UserID][client.ID] = client
			h.mu.Unlock()
			_ = h.Svc.SetPresence(context.Background(), client.UserID, constants.PresenceOnline, "websocket")
		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[client.ID]; ok {
				delete(h.Clients, client.ID)
				delete(h.UserIndex[client.UserID], client.ID)
				if len(h.UserIndex[client.UserID]) == 0 {
					delete(h.UserIndex, client.UserID)
					go func(uid uuid.UUID) {
						_ = h.Svc.SetPresence(context.Background(), uid, constants.PresenceOffline, "websocket")
					}(client.UserID)
				}
				close(client.Send)
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) Publish(ctx context.Context, evt events.Event) error {
	data, err := evt.Bytes()
	if err != nil {
		return err
	}
	channel := constants.RedisChannelPrefix + "global"
	if evt.ConversationID != nil {
		channel = constants.RedisChannelPrefix + evt.ConversationID.String()
	}
	return h.RDB.Publish(ctx, channel, data).Err()
}

func (h *Hub) subscribeRedis(ctx context.Context) {
	pubsub := h.RDB.PSubscribe(ctx, constants.RedisChannelPrefix+"*")
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
			h.dispatch(msg.Payload)
		}
	}
}

func (h *Hub) dispatch(payload string) {
	evt, err := events.Parse([]byte(payload))
	if err != nil {
		return
	}
	data := []byte(payload)
	h.mu.RLock()
	defer h.mu.RUnlock()
	if evt.ConversationID != nil {
		for _, client := range h.Clients {
			client.mu.Lock()
			_, inRoom := client.Rooms[*evt.ConversationID]
			client.mu.Unlock()
			if inRoom || evt.Type == constants.WSEventPresence || evt.Type == constants.WSEventNotification {
				select {
				case client.Send <- data:
				default:
				}
			}
		}
		return
	}
	if evt.UserID != nil {
		for _, client := range h.UserIndex[*evt.UserID] {
			select {
			case client.Send <- data:
			default:
			}
		}
		// also broadcast presence to all
		if evt.Type == constants.WSEventPresence {
			for _, client := range h.Clients {
				select {
				case client.Send <- data:
				default:
				}
			}
		}
	}
}

type inbound struct {
	Type           string          `json:"type"`
	ConversationID *uuid.UUID      `json:"conversationId"`
	Payload        json.RawMessage `json:"payload"`
}

func (h *Hub) HandleWS(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		auth := c.GetHeader("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimPrefix(auth, "Bearer ")
		}
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
		slog.Error("websocket upgrade failed", "error", err)
		return
	}
	client := &Client{
		ID: uuid.New(), UserID: claims.UserID, Hub: h, Conn: conn,
		Send: make(chan []byte, 256), Rooms: map[uuid.UUID]struct{}{},
	}
	h.Register <- client
	go client.writePump()
	go client.readPump()
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
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}
		var in inbound
		if err := json.Unmarshal(message, &in); err != nil {
			continue
		}
		ctx := context.Background()
		switch in.Type {
		case constants.WSEventPing:
			pong, _ := events.New(constants.WSEventPong, nil, &c.UserID, map[string]string{"ok": "true"})
			b, _ := pong.Bytes()
			c.Send <- b
		case constants.WSEventJoin:
			if in.ConversationID != nil {
				ok, err := c.Hub.Svc.Conversations.IsMember(ctx, *in.ConversationID, c.UserID)
				if err == nil && ok {
					c.mu.Lock()
					c.Rooms[*in.ConversationID] = struct{}{}
					c.mu.Unlock()
					go c.Hub.Svc.MarkDeliveredOnJoin(context.Background(), c.UserID, *in.ConversationID)
				}
			}
		case constants.WSEventLeave:
			if in.ConversationID != nil {
				c.mu.Lock()
				delete(c.Rooms, *in.ConversationID)
				c.mu.Unlock()
			}
		case constants.WSEventTyping, constants.WSEventRecording:
			if in.ConversationID == nil {
				continue
			}
			var p struct {
				IsTyping    bool `json:"isTyping"`
				IsRecording bool `json:"isRecording"`
			}
			_ = json.Unmarshal(in.Payload, &p)
			_ = c.Hub.Svc.SetTyping(ctx, c.UserID, *in.ConversationID, p.IsTyping, p.IsRecording)
		case constants.WSEventPresence:
			var p struct {
				Status string `json:"status"`
			}
			_ = json.Unmarshal(in.Payload, &p)
			if p.Status != "" {
				_ = c.Hub.Svc.SetPresence(ctx, c.UserID, p.Status, "websocket")
			}
		case constants.WSEventReceipt:
			if in.ConversationID == nil {
				continue
			}
			var p struct {
				MessageIDs []uuid.UUID `json:"messageIds"`
				Status     string      `json:"status"`
			}
			_ = json.Unmarshal(in.Payload, &p)
			_ = c.Hub.Svc.MarkReceipts(ctx, c.UserID, *in.ConversationID, p.MessageIDs, p.Status)
		}
	}
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
