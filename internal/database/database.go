package database

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/pulse/chat-service/internal/models"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func ConnectPostgres(dsn string) (*gorm.DB, error) {
	cfg := &gorm.Config{
		Logger: logger.New(slog.NewLogLogger(slog.Default().Handler(), slog.LevelWarn), logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		}),
		NowFunc: func() time.Time { return time.Now().UTC() },
	}
	db, err := gorm.Open(postgres.Open(dsn), cfg)
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}

func ConnectRedis(redisURL string) (*redis.Client, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return client, nil
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.RefreshToken{},
		&models.Conversation{},
		&models.ConversationMember{},
		&models.Message{},
		&models.Attachment{},
		&models.Reaction{},
		&models.ReadReceipt{},
		&models.MessageStatus{},
		&models.TypingStatus{},
		&models.PinnedMessage{},
		&models.MessageEdit{},
		&models.VoiceMessage{},
		&models.DraftMessage{},
		&models.Mention{},
		&models.DeletedMessage{},
		&models.ArchivedChat{},
		&models.MutedChat{},
		&models.BlockedUser{},
		&models.NotificationQueue{},
		&models.CallHistory{},
		&models.UserPresence{},
		&models.DeviceToken{},
		&models.ChatSettings{},
		&models.StarredMessage{},
	)
}

func EnsureUploadDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func HealthCheck(ctx context.Context, db *gorm.DB, rdb *redis.Client) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	return nil
}
