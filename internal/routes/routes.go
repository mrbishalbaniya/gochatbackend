package routes

import (
	"github.com/gin-gonic/gin"
	callhandlers "github.com/pulse/chat-service/internal/call/handlers"
	callws "github.com/pulse/chat-service/internal/call/websocket"
	"github.com/pulse/chat-service/internal/config"
	"github.com/pulse/chat-service/internal/handlers"
	"github.com/pulse/chat-service/internal/middleware"
	ws "github.com/pulse/chat-service/internal/websocket"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func Register(
	r *gin.Engine,
	cfg *config.Config,
	h *handlers.Handler,
	hub *ws.Hub,
	callH *callhandlers.Handler,
	callHub *callws.Hub,
) {
	r.Use(middleware.RequestID(), middleware.SecurityHeaders(), middleware.CORS(cfg.CORSOrigins), middleware.RateLimit(cfg.RateLimitRPM))

	r.GET("/health", h.Health)
	r.GET("/ready", h.Ready)
	r.GET("/metrics", h.Metrics)
	r.Static("/uploads", cfg.UploadDir)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/ws", hub.HandleWS)
	r.GET("/ws/calls", callHub.HandleWS)

	v1 := r.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		{
			auth.POST("/register", h.Register)
			auth.POST("/login", h.Login)
			auth.POST("/refresh", h.Refresh)
			auth.POST("/logout", h.Logout)
		}

		v1.GET("/ice-servers", callH.ICEServers)

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
			secured.GET("/push/vapid-public-key", h.VAPIDPublicKey)
			secured.POST("/push/subscribe", h.PushSubscribe)
			secured.DELETE("/push/subscribe", h.PushUnsubscribe)

			// WebRTC calling
			secured.POST("/calls", callH.StartCall)
			secured.GET("/calls", callH.History)
			secured.GET("/calls/missed", callH.Missed)
			secured.GET("/calls/settings", callH.GetSettings)
			secured.PATCH("/calls/settings", callH.UpdateSettings)
			secured.GET("/calls/:id", callH.GetCall)
			secured.POST("/calls/:id/accept", callH.Accept)
			secured.POST("/calls/:id/reject", callH.Reject)
			secured.POST("/calls/:id/end", callH.End)
			secured.POST("/calls/:id/invite", callH.Invite)
			secured.POST("/calls/:id/quality", callH.Quality)
			secured.PATCH("/calls/:id/media", callH.MediaState)
			secured.GET("/ice-servers/auth", callH.ICEServers)
		}
	}
}
