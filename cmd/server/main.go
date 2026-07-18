package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	callhandlers "github.com/pulse/chat-service/internal/call/handlers"
	callservices "github.com/pulse/chat-service/internal/call/services"
	callws "github.com/pulse/chat-service/internal/call/websocket"
	"github.com/pulse/chat-service/internal/config"
	"github.com/pulse/chat-service/internal/database"
	"github.com/pulse/chat-service/internal/handlers"
	"github.com/pulse/chat-service/internal/routes"
	"github.com/pulse/chat-service/internal/services"
	"github.com/pulse/chat-service/internal/storage"
	"github.com/pulse/chat-service/internal/workers"
	ws "github.com/pulse/chat-service/internal/websocket"

	_ "github.com/pulse/chat-service/docs"
)

// @title Pulse Chat Service API
// @version 1.0
// @description Real-time messaging + WebRTC calling API for Pulse
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	db, err := database.ConnectPostgres(cfg.DatabaseURL)
	if err != nil {
		slog.Error("postgres connect failed", "error", err)
		os.Exit(1)
	}
	if err := database.AutoMigrate(db); err != nil {
		slog.Error("migrate failed", "error", err)
		os.Exit(1)
	}
	rdb, err := database.ConnectRedis(cfg.RedisURL)
	if err != nil {
		slog.Error("redis connect failed", "error", err)
		os.Exit(1)
	}
	if err := database.EnsureUploadDir(cfg.UploadDir); err != nil {
		slog.Error("upload dir failed", "error", err)
		os.Exit(1)
	}

	store := &storage.LocalStorage{Root: cfg.UploadDir, PublicBase: cfg.PublicBaseURL, MaxBytes: cfg.MaxUploadBytes}
	hub := ws.NewHub(rdb, nil, cfg.JWTAccessSecret, cfg.CORSOrigins)
	svc := services.New(cfg, db, rdb, store, hub)
	hub.Svc = svc

	callHub := callws.NewHub(rdb, nil, cfg.JWTAccessSecret, cfg.CORSOrigins)
	callSvc := callservices.New(cfg, db, rdb, callHub)
	callHub.Svc = callSvc
	callHandler := callhandlers.New(callSvc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)
	go callHub.Run(ctx)
	go (&workers.NotificationWorker{DB: db}).Run(ctx)

	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())
	h := handlers.New(svc)
	routes.Register(r, cfg, h, hub, callHandler, callHub)

	srv := &http.Server{Addr: ":" + cfg.HTTPPort, Handler: r, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		slog.Info("service listening", "app", cfg.AppName, "port", cfg.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}
