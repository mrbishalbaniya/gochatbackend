package handlers

import (
	"strconv"
	"time"

	"github.com/pulse/chat-service/internal/dto"
	"github.com/pulse/chat-service/internal/middleware"
	"github.com/pulse/chat-service/internal/services"
	"github.com/pulse/chat-service/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	Svc *services.Services
}

func New(svc *services.Services) *Handler {
	return &Handler{Svc: svc}
}

func (h *Handler) Health(c *gin.Context) {
	utils.JSON(c, 200, gin.H{"status": "ok", "service": "pulse-chat-service"})
}

func (h *Handler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	res, err := h.Svc.Register(c.Request.Context(), req)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.JSON(c, 201, res)
}

func (h *Handler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	res, err := h.Svc.Login(c.Request.Context(), req, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		utils.Unauthorized(c, err.Error())
		return
	}
	utils.JSON(c, 200, res)
}

func (h *Handler) Refresh(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	res, err := h.Svc.Refresh(c.Request.Context(), req.RefreshToken, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		utils.Unauthorized(c, err.Error())
		return
	}
	utils.JSON(c, 200, res)
}

func (h *Handler) Logout(c *gin.Context) {
	var req dto.RefreshRequest
	_ = c.ShouldBindJSON(&req)
	_ = h.Svc.Logout(c.Request.Context(), req.RefreshToken)
	utils.JSON(c, 200, gin.H{"ok": true})
}

func (h *Handler) Me(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	user, err := h.Svc.GetProfile(c.Request.Context(), uid)
	if err != nil {
		utils.NotFound(c, "user not found")
		return
	}
	utils.JSON(c, 200, user)
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	var req dto.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	user, err := h.Svc.UpdateProfile(c.Request.Context(), uid, req)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.JSON(c, 200, user)
}

func (h *Handler) CreateConversation(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	var req dto.CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	view, err := h.Svc.CreateConversation(c.Request.Context(), uid, req)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.JSON(c, 201, view)
}

func (h *Handler) ListConversations(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	archived := c.Query("archived") == "true"
	list, err := h.Svc.ListConversations(c.Request.Context(), uid, archived)
	if err != nil {
		utils.Internal(c, err.Error())
		return
	}
	utils.JSON(c, 200, list)
}

func (h *Handler) GetConversation(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	view, err := h.Svc.GetConversation(c.Request.Context(), uid, id)
	if err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, view)
}

func (h *Handler) ArchiveConversation(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	archived := c.Query("value") != "false"
	if err := h.Svc.SetArchived(c.Request.Context(), uid, id, archived); err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, gin.H{"archived": archived})
}

func (h *Handler) MuteConversation(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	var req dto.MuteRequest
	_ = c.ShouldBindJSON(&req)
	mute := c.Query("value") != "false"
	if err := h.Svc.SetMuted(c.Request.Context(), uid, id, mute, req.MuteUntil); err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, gin.H{"muted": mute})
}

func (h *Handler) PinConversation(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	pinned := c.Query("value") != "false"
	if err := h.Svc.SetPinnedConversation(c.Request.Context(), uid, id, pinned); err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, gin.H{"pinned": pinned})
}

func (h *Handler) AddMembers(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	var body struct {
		MemberIDs []uuid.UUID `json:"memberIds"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := h.Svc.AddMembers(c.Request.Context(), uid, id, body.MemberIDs); err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, gin.H{"ok": true})
}

func (h *Handler) SendMessage(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	var req dto.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	view, err := h.Svc.SendMessage(c.Request.Context(), uid, id, req)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.JSON(c, 201, view)
}

func (h *Handler) ListMessages(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "40"))
	var before *time.Time
	if b := c.Query("before"); b != "" {
		t, err := time.Parse(time.RFC3339Nano, b)
		if err == nil {
			before = &t
		}
	}
	views, meta, err := h.Svc.ListMessages(c.Request.Context(), uid, id, before, limit)
	if err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSONMeta(c, 200, views, meta)
}

func (h *Handler) EditMessage(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	var req dto.EditMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	view, err := h.Svc.EditMessage(c.Request.Context(), uid, id, req.Body)
	if err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, view)
}

func (h *Handler) DeleteMessage(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	forEveryone := c.Query("forEveryone") == "true"
	if err := h.Svc.DeleteMessage(c.Request.Context(), uid, id, forEveryone); err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, gin.H{"deleted": true})
}

func (h *Handler) React(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	var req dto.ReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	remove := c.Query("remove") == "true"
	if err := h.Svc.React(c.Request.Context(), uid, id, req.Emoji, remove); err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, gin.H{"ok": true})
}

func (h *Handler) Receipts(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	var req dto.ReceiptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := h.Svc.MarkReceipts(c.Request.Context(), uid, id, req.MessageIDs, req.Status); err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, gin.H{"ok": true})
}

func (h *Handler) Typing(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	var req dto.TypingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := h.Svc.SetTyping(c.Request.Context(), uid, id, req.IsTyping, req.IsRecording); err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, gin.H{"ok": true})
}

func (h *Handler) Presence(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	var req dto.PresenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := h.Svc.SetPresence(c.Request.Context(), uid, req.Status, "api"); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.JSON(c, 200, gin.H{"status": req.Status})
}

func (h *Handler) PinMessage(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	cid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid conversation id")
		return
	}
	mid, err := uuid.Parse(c.Param("messageId"))
	if err != nil {
		utils.BadRequest(c, "invalid message id")
		return
	}
	pin := c.Query("value") != "false"
	if err := h.Svc.PinMessage(c.Request.Context(), uid, cid, mid, pin); err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, gin.H{"pinned": pin})
}

func (h *Handler) ListPinned(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	cid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	items, err := h.Svc.ListPinned(c.Request.Context(), uid, cid)
	if err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, items)
}

func (h *Handler) Search(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	var q dto.SearchQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	res, err := h.Svc.Search(c.Request.Context(), uid, q.Q, q.Type, q.ConversationID, q.Limit)
	if err != nil {
		utils.Internal(c, err.Error())
		return
	}
	utils.JSON(c, 200, res)
}

func (h *Handler) Upload(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	cid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		utils.BadRequest(c, "file required")
		return
	}
	f, err := file.Open()
	if err != nil {
		utils.Internal(c, err.Error())
		return
	}
	defer f.Close()
	duration, _ := strconv.Atoi(c.PostForm("durationMs"))
	view, err := h.Svc.UploadAttachment(c.Request.Context(), uid, cid, file.Filename, f, file.Size, file.Header.Get("Content-Type"), c.PostForm("body"), c.PostForm("type"), duration, c.PostForm("waveform"))
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.JSON(c, 201, view)
}

func (h *Handler) SaveDraft(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	cid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	var req dto.DraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	d, err := h.Svc.SaveDraft(c.Request.Context(), uid, cid, req)
	if err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, d)
}

func (h *Handler) GetDraft(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	cid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	d, err := h.Svc.GetDraft(c.Request.Context(), uid, cid)
	if err != nil {
		utils.JSON(c, 200, gin.H{"body": ""})
		return
	}
	utils.JSON(c, 200, d)
}

func (h *Handler) Block(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	var req dto.BlockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := h.Svc.BlockUser(c.Request.Context(), uid, req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.JSON(c, 200, gin.H{"blocked": true})
}

func (h *Handler) Unblock(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	_ = h.Svc.UnblockUser(c.Request.Context(), uid, id)
	utils.JSON(c, 200, gin.H{"unblocked": true})
}

func (h *Handler) ListBlocked(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	items, err := h.Svc.ListBlocked(c.Request.Context(), uid)
	if err != nil {
		utils.Internal(c, err.Error())
		return
	}
	utils.JSON(c, 200, items)
}

func (h *Handler) GetSettings(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	s, err := h.Svc.GetSettings(c.Request.Context(), uid)
	if err != nil {
		utils.Internal(c, err.Error())
		return
	}
	utils.JSON(c, 200, s)
}

func (h *Handler) UpdateSettings(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	var req dto.SettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	s, err := h.Svc.UpdateSettings(c.Request.Context(), uid, req)
	if err != nil {
		utils.Internal(c, err.Error())
		return
	}
	utils.JSON(c, 200, s)
}

func (h *Handler) DeviceToken(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	var req dto.DeviceTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := h.Svc.RegisterDevice(c.Request.Context(), uid, req); err != nil {
		utils.Internal(c, err.Error())
		return
	}
	utils.JSON(c, 200, gin.H{"ok": true})
}

func (h *Handler) VAPIDPublicKey(c *gin.Context) {
	key := h.Svc.Cfg.VAPIDPublicKey
	if key == "" {
		utils.JSON(c, 200, gin.H{"publicKey": "", "enabled": false})
		return
	}
	utils.JSON(c, 200, gin.H{"publicKey": key, "enabled": true})
}

func (h *Handler) PushSubscribe(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	var req dto.PushSubscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := h.Svc.SubscribePush(c.Request.Context(), uid, req); err != nil {
		utils.Internal(c, err.Error())
		return
	}
	utils.JSON(c, 200, gin.H{"ok": true})
}

func (h *Handler) PushUnsubscribe(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	var body struct {
		Endpoint string `json:"endpoint" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := h.Svc.UnsubscribePush(c.Request.Context(), uid, body.Endpoint); err != nil {
		utils.Internal(c, err.Error())
		return
	}
	utils.JSON(c, 200, gin.H{"ok": true})
}

func (h *Handler) Notifications(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	items, err := h.Svc.ListNotifications(c.Request.Context(), uid)
	if err != nil {
		utils.Internal(c, err.Error())
		return
	}
	utils.JSON(c, 200, items)
}

func (h *Handler) StartCall(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	cid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	var req dto.CallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	call, err := h.Svc.StartCall(c.Request.Context(), uid, cid, req.CallType)
	if err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 201, call)
}

func (h *Handler) EndCall(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	call, err := h.Svc.EndCall(c.Request.Context(), uid, id)
	if err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, call)
}

func (h *Handler) ListCalls(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	items, err := h.Svc.ListCalls(c.Request.Context(), uid)
	if err != nil {
		utils.Internal(c, err.Error())
		return
	}
	utils.JSON(c, 200, items)
}

func (h *Handler) Star(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	star := c.Query("value") != "false"
	if err := h.Svc.StarMessage(c.Request.Context(), uid, id, star); err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, gin.H{"starred": star})
}

func (h *Handler) ListStarred(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	items, err := h.Svc.ListStarred(c.Request.Context(), uid)
	if err != nil {
		utils.Internal(c, err.Error())
		return
	}
	utils.JSON(c, 200, items)
}

func (h *Handler) Forward(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	var body struct {
		TargetConversationID uuid.UUID   `json:"targetConversationId"`
		MessageIDs           []uuid.UUID `json:"messageIds"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	views, err := h.Svc.ForwardMessages(c.Request.Context(), uid, body.TargetConversationID, body.MessageIDs)
	if err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 201, views)
}

func (h *Handler) SearchUsers(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	_ = uid
	q := c.Query("q")
	users, err := h.Svc.Users.Search(c.Request.Context(), q, 20)
	if err != nil {
		utils.Internal(c, err.Error())
		return
	}
	out := make([]dto.UserDTO, 0, len(users))
	for i := range users {
		out = append(out, dto.UserDTO{
			ID: users[i].ID, Email: users[i].Email, Username: users[i].Username,
			DisplayName: users[i].DisplayName, AvatarURL: users[i].AvatarURL, Bio: users[i].Bio, Role: users[i].Role,
		})
	}
	utils.JSON(c, 200, out)
}

func (h *Handler) ClearHistory(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	if err := h.Svc.ClearHistory(c.Request.Context(), uid, id); err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, gin.H{"cleared": true})
}

func (h *Handler) WSTicket(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	ticket, err := h.Svc.IssueWSTicket(c.Request.Context(), uid, id)
	if err != nil {
		utils.Forbidden(c, err.Error())
		return
	}
	utils.JSON(c, 200, gin.H{"ticket": ticket})
}
