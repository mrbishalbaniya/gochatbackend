package workers

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/pulse/chat-service/internal/models"
	"github.com/pulse/chat-service/internal/push"
	"gorm.io/gorm"
)

// NotificationWorker delivers pending notifications via Web Push.
type NotificationWorker struct {
	DB       *gorm.DB
	Push     *push.Sender
	Interval time.Duration
}

func (w *NotificationWorker) Run(ctx context.Context) {
	if w.Interval == 0 {
		w.Interval = 3 * time.Second
	}
	t := time.NewTicker(w.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.flush(ctx)
		}
	}
}

func (w *NotificationWorker) flush(ctx context.Context) {
	var pending []models.NotificationQueue
	if err := w.DB.WithContext(ctx).Where("status = ?", "pending").Order("created_at ASC").Limit(100).Find(&pending).Error; err != nil {
		slog.Error("notification worker query failed", "error", err)
		return
	}
	now := time.Now().UTC()
	for i := range pending {
		n := &pending[i]
		if w.Push != nil && w.Push.Enabled() {
			data := map[string]interface{}{}
			_ = json.Unmarshal(n.Payload, &data)
			url := "/inbox"
			if cid, ok := data["conversationId"].(string); ok && cid != "" {
				url = "/inbox?c=" + cid
			}
			if path, ok := data["url"].(string); ok && path != "" {
				url = path
			}
			w.Push.SendToUser(ctx, n.UserID, push.Payload{
				Title: n.Title,
				Body:  n.Body,
				Type:  n.Type,
				Tag:   n.Type + "-" + n.ID.String(),
				URL:   url,
				Data:  data,
			})
		}
		n.Status = "sent"
		n.SentAt = &now
		if err := w.DB.WithContext(ctx).Save(n).Error; err != nil {
			slog.Error("notification mark sent failed", "error", err)
		}
	}
}
