package workers

import (
	"context"
	"log/slog"
	"time"

	"github.com/pulse/chat-service/internal/models"
	"gorm.io/gorm"
)

// NotificationWorker marks pending notifications as sent (hook for FCM/Web Push).
type NotificationWorker struct {
	DB       *gorm.DB
	Interval time.Duration
}

func (w *NotificationWorker) Run(ctx context.Context) {
	if w.Interval == 0 {
		w.Interval = 5 * time.Second
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
		pending[i].Status = "sent"
		pending[i].SentAt = &now
		if err := w.DB.WithContext(ctx).Save(&pending[i]).Error; err != nil {
			slog.Error("notification mark sent failed", "error", err)
		}
	}
}
