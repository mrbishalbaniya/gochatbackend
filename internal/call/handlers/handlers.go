package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/pulse/chat-service/internal/call/dto"
	"github.com/pulse/chat-service/internal/middleware"
	"github.com/pulse/chat-service/internal/call/services"
	"github.com/pulse/chat-service/internal/utils"
)

type Handler struct {
	Svc *services.Services
}

func New(svc *services.Services) *Handler { return &Handler{Svc: svc} }

func (h *Handler) Health(c *gin.Context) {
	utils.JSON(c, 200, gin.H{"status": "ok", "service": "pulse-call-service"})
}

func (h *Handler) Ready(c *gin.Context) {
	utils.JSON(c, 200, gin.H{"ready": true})
}

func (h *Handler) Metrics(c *gin.Context) {
	promhttp.Handler().ServeHTTP(c.Writer, c.Request)
}

func (h *Handler) ICEServers(c *gin.Context) {
	utils.JSON(c, 200, h.Svc.ICEConfig())
}

func (h *Handler) StartCall(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	var req dto.StartCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	view, err := h.Svc.StartCall(c.Request.Context(), uid, req)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.JSON(c, 201, view)
}

func (h *Handler) GetCall(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	view, err := h.Svc.GetCall(c.Request.Context(), uid, id)
	if err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, view)
}

func (h *Handler) Accept(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	view, err := h.Svc.AcceptCall(c.Request.Context(), uid, id)
	if err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, view)
}

func (h *Handler) Reject(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	if err := h.Svc.RejectCall(c.Request.Context(), uid, id); err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, gin.H{"rejected": true})
}

func (h *Handler) End(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	view, err := h.Svc.EndCall(c.Request.Context(), uid, id)
	if err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, view)
}

func (h *Handler) Invite(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	var req dto.InviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := h.Svc.Invite(c.Request.Context(), uid, id, req.UserIDs); err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, gin.H{"ok": true})
}

func (h *Handler) Quality(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	var req dto.QualityReport
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := h.Svc.ReportQuality(c.Request.Context(), uid, id, req); err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, gin.H{"ok": true})
}

func (h *Handler) History(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	items, err := h.Svc.ListHistory(c.Request.Context(), uid)
	if err != nil {
		utils.Internal(c, err.Error())
		return
	}
	utils.JSON(c, 200, items)
}

func (h *Handler) Missed(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	items, err := h.Svc.ListMissed(c.Request.Context(), uid)
	if err != nil {
		utils.Internal(c, err.Error())
		return
	}
	utils.JSON(c, 200, items)
}

func (h *Handler) GetSettings(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	st, err := h.Svc.GetOrCreateSettings(c.Request.Context(), uid)
	if err != nil {
		utils.Internal(c, err.Error())
		return
	}
	utils.JSON(c, 200, st)
}

func (h *Handler) UpdateSettings(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	var req dto.SettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	st, err := h.Svc.UpdateSettings(c.Request.Context(), uid, req)
	if err != nil {
		utils.Internal(c, err.Error())
		return
	}
	utils.JSON(c, 200, st)
}

func (h *Handler) MediaState(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	var body struct {
		Muted       *bool `json:"muted"`
		CameraOn    *bool `json:"cameraOn"`
		ScreenShare *bool `json:"screenShare"`
		RaisedHand  *bool `json:"raisedHand"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	p, err := h.Svc.UpdateMediaState(c.Request.Context(), uid, id, body.Muted, body.CameraOn, body.ScreenShare, body.RaisedHand)
	if err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, p)
}
