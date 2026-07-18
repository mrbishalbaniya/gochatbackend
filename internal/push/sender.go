package push

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/pulse/chat-service/internal/config"
	"github.com/pulse/chat-service/internal/models"
	"gorm.io/gorm"
)

type Sender struct {
	Cfg *config.Config
	DB  *gorm.DB
}

type Payload struct {
	Title string                 `json:"title"`
	Body  string                 `json:"body"`
	URL   string                 `json:"url,omitempty"`
	Tag   string                 `json:"tag,omitempty"`
	Type  string                 `json:"type,omitempty"`
	Data  map[string]interface{} `json:"data,omitempty"`
	Icon  string                 `json:"icon,omitempty"`
	Badge string                 `json:"badge,omitempty"`
}

func (s *Sender) Enabled() bool {
	return s != nil && s.Cfg != nil && s.Cfg.VAPIDPublicKey != "" && s.Cfg.VAPIDPrivateKey != ""
}

func (s *Sender) SendToUser(ctx context.Context, userID uuid.UUID, p Payload) {
	if !s.Enabled() {
		return
	}
	var devices []models.DeviceToken
	if err := s.DB.WithContext(ctx).
		Where("user_id = ? AND active = true AND platform = ? AND p256dh <> '' AND auth <> ''", userID, "web").
		Find(&devices).Error; err != nil {
		slog.Error("push list devices", "error", err)
		return
	}
	for i := range devices {
		s.sendOne(ctx, &devices[i], p)
	}
}

func (s *Sender) sendOne(ctx context.Context, d *models.DeviceToken, p Payload) {
	if p.Icon == "" {
		p.Icon = "/icon.svg"
	}
	body, err := json.Marshal(p)
	if err != nil {
		return
	}
	sub := &webpush.Subscription{
		Endpoint: d.Token,
		Keys: webpush.Keys{
			P256dh: d.P256dh,
			Auth:   d.Auth,
		},
	}
	resp, err := webpush.SendNotificationWithContext(ctx, body, sub, &webpush.Options{
		Subscriber:      s.Cfg.VAPIDSubject,
		VAPIDPublicKey:  s.Cfg.VAPIDPublicKey,
		VAPIDPrivateKey: s.Cfg.VAPIDPrivateKey,
		TTL:             60,
		Urgency:         webpush.UrgencyHigh,
	})
	if err != nil {
		slog.Warn("web push send failed", "error", err, "userId", d.UserID)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 || resp.StatusCode == 410 {
		_ = s.DB.WithContext(ctx).Model(d).Updates(map[string]interface{}{"active": false, "updated_at": time.Now().UTC()}).Error
	}
}
