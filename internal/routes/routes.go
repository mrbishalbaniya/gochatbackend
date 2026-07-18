package routes

import (
	"github.com/pulse/chat-service/internal/config"
	"github.com/pulse/chat-service/internal/handlers"
	"github.com/pulse/chat-service/internal/middleware"
	ws "github.com/pulse/chat-service/internal/websocket"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func Register(r *gin.Engine, cfg *config.Config, h *handlers.Handler, hub *ws.Hub) {
	r.Use(middleware.RequestID(), middleware.SecurityHeaders(), middleware.CORS(cfg.CORSOrigins), middleware.RateLimit(cfg.RateLimitRPM))

	r.GET("/health", h.Health)
	r.GET("/ready", h.Ready)
	r.GET("/metrics", h.Metrics)
	r.Static("/uploads", cfg.UploadDir)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/ws", hub.HandleWS)

	v1 := r.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		{
			auth.POST("/register", h.Register)
			auth.POST("/login", h.Login)
			auth.POST("/refresh", h.Refresh)
			auth.POST("/logout", h.Logout)
		}

		secured := v1.Group("")
		secured.Use(middleware.AuthJWT(cfg.JWTAccessSecret))
		{
			secured.GET("/me", h.Me)
			secured.PATCH("/me", h.UpdateProfile)
			secured.GET("/users/search", h.SearchUsers)

			secured.POST("/conversations", h.CreateConversation)
			secured.GET("/conversations", h.ListConversations)
			secured.GET("/conversations/:id", h.GetConversation)
			secured.POST("/conversations/:id/archive", h.ArchiveConversation)
			secured.POST("/conversations/:id/mute", h.MuteConversation)
			secured.POST("/conversations/:id/pin", h.PinConversation)
			secured.POST("/conversations/:id/members", h.AddMembers)
			secured.POST("/conversations/:id/messages", h.SendMessage)
			secured.GET("/conversations/:id/messages", h.ListMessages)
			secured.POST("/conversations/:id/typing", h.Typing)
			secured.POST("/conversations/:id/receipts", h.Receipts)
			secured.POST("/conversations/:id/upload", h.Upload)
			secured.PUT("/conversations/:id/draft", h.SaveDraft)
			secured.GET("/conversations/:id/draft", h.GetDraft)
			secured.POST("/conversations/:id/clear", h.ClearHistory)
			secured.POST("/conversations/:id/ws-ticket", h.WSTicket)
			secured.POST("/conversations/:id/messages/:messageId/pin", h.PinMessage)
			secured.GET("/conversations/:id/pins", h.ListPinned)
			secured.POST("/conversations/:id/calls", h.StartCall)

			secured.PATCH("/messages/:id", h.EditMessage)
			secured.DELETE("/messages/:id", h.DeleteMessage)
			secured.POST("/messages/:id/react", h.React)
			secured.POST("/messages/:id/star", h.Star)
			secured.POST("/messages/forward", h.Forward)
			secured.GET("/messages/starred", h.ListStarred)

			secured.POST("/presence", h.Presence)
			secured.GET("/search", h.Search)

			secured.POST("/block", h.Block)
			secured.DELETE("/block/:id", h.Unblock)
			secured.GET("/block", h.ListBlocked)

			secured.GET("/settings", h.GetSettings)
			secured.PATCH("/settings", h.UpdateSettings)
			secured.POST("/devices", h.DeviceToken)
			secured.GET("/notifications", h.Notifications)
			secured.GET("/calls", h.ListCalls)
			secured.POST("/calls/:id/end", h.EndCall)
		}
	}
}
