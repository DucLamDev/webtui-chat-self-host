package http

import (
	"encoding/json"
	"log/slog"
	nethttp "net/http"
	"net/url"
	"strconv"

	messagesapp "github.com/duclamdev/application-chat/backend/internal/modules/messages/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *messagesapp.Service
}

type sendMessageRequest struct {
	ParentID         string          `json:"parent_id"`
	ClientMessageID  string          `json:"client_message_id"`
	Kind             string          `json:"kind"`
	Body             string          `json:"body"`
	Metadata         json.RawMessage `json:"metadata"`
	MentionedUserIDs []string        `json:"mentioned_user_ids"`
}

type updateMessageRequest struct {
	Body string `json:"body"`
}

type reactionRequest struct {
	Emoji string `json:"emoji"`
}

type forwardMessageRequest struct {
	TargetChannelID string `json:"target_channel_id"`
}

func NewHandler(service *messagesapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	private := router.Group("/workspaces/:workspace_id")
	private.Use(authMiddleware)

	private.GET("/messages/search", h.Search)
	private.GET("/channels/:channel_id/messages", h.List)
	private.POST("/channels/:channel_id/messages", h.Send)
	private.GET("/channels/:channel_id/pins", h.ListPins)
	private.GET("/channels/:channel_id/messages/:message_id", h.Get)
	private.PATCH("/channels/:channel_id/messages/:message_id", h.Update)
	private.DELETE("/channels/:channel_id/messages/:message_id", h.Delete)
	private.POST("/channels/:channel_id/messages/:message_id/forward", h.Forward)
	private.POST("/channels/:channel_id/messages/:message_id/pin", h.Pin)
	private.DELETE("/channels/:channel_id/messages/:message_id/pin", h.Unpin)
	private.GET("/channels/:channel_id/messages/:message_id/thread", h.ListThread)
	private.POST("/channels/:channel_id/messages/:message_id/reactions", h.AddReaction)
	private.DELETE("/channels/:channel_id/messages/:message_id/reactions/:emoji", h.RemoveReaction)
}

func (h *Handler) Forward(c *gin.Context) {
	var req forwardMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	message, err := h.service.Forward(c.Request.Context(), messagesapp.ForwardInput{
		ActorUserID:     middleware.CurrentUserID(c),
		WorkspaceID:     c.Param("workspace_id"),
		ChannelID:       c.Param("channel_id"),
		MessageID:       c.Param("message_id"),
		TargetChannelID: req.TargetChannelID,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, message)
}

func (h *Handler) Send(c *gin.Context) {
	var req sendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}

	slog.Info("Nhan request gui tin nhan",
		"workspace_id", c.Param("workspace_id"),
		"channel_id", c.Param("channel_id"),
		"actor_user_id", middleware.CurrentUserID(c),
		"request_id", middleware.GetRequestID(c),
		"kind", req.Kind,
		"body_len", len([]rune(req.Body)),
		"has_parent", req.ParentID != "",
		"mention_count", len(req.MentionedUserIDs),
	)
	message, err := h.service.Send(c.Request.Context(), messagesapp.SendInput{
		ActorUserID:      middleware.CurrentUserID(c),
		WorkspaceID:      c.Param("workspace_id"),
		ChannelID:        c.Param("channel_id"),
		ParentID:         req.ParentID,
		ClientMessageID:  firstNonEmpty(req.ClientMessageID, c.GetHeader("Idempotency-Key")),
		Kind:             req.Kind,
		Body:             req.Body,
		Metadata:         req.Metadata,
		MentionedUserIDs: req.MentionedUserIDs,
	})
	if err != nil {
		slog.Warn("Gui tin nhan that bai",
			"workspace_id", c.Param("workspace_id"),
			"channel_id", c.Param("channel_id"),
			"actor_user_id", middleware.CurrentUserID(c),
			"request_id", middleware.GetRequestID(c),
			"error", err,
		)
		response.Error(c, err)
		return
	}
	slog.Info("Gui tin nhan thanh cong",
		"workspace_id", message.WorkspaceID,
		"channel_id", message.ChannelID,
		"message_id", message.ID,
		"request_id", middleware.GetRequestID(c),
		"kind", message.Kind,
	)
	response.Created(c, message)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (h *Handler) Get(c *gin.Context) {
	message, err := h.service.Get(c.Request.Context(), messagesapp.MessageRef{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		ChannelID:   c.Param("channel_id"),
		MessageID:   c.Param("message_id"),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, message)
}

func (h *Handler) List(c *gin.Context) {
	messages, meta, err := h.service.List(c.Request.Context(), messagesapp.ListInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		ChannelID:   c.Param("channel_id"),
		Limit:       queryInt(c, "limit"),
		BeforeID:    c.Query("before"),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OKWithMeta(c, nethttp.StatusOK, gin.H{"messages": messages}, meta)
}

func (h *Handler) ListThread(c *gin.Context) {
	messages, meta, err := h.service.ListThread(c.Request.Context(), messagesapp.ThreadInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		ChannelID:   c.Param("channel_id"),
		MessageID:   c.Param("message_id"),
		Limit:       queryInt(c, "limit"),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OKWithMeta(c, nethttp.StatusOK, gin.H{"messages": messages}, meta)
}

func (h *Handler) Search(c *gin.Context) {
	messages, meta, err := h.service.Search(c.Request.Context(), messagesapp.SearchInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		Query:       c.Query("q"),
		ChannelID:   c.Query("channel_id"),
		SenderID:    c.Query("sender_id"),
		Kind:        c.Query("kind"),
		DateFrom:    c.Query("date_from"),
		DateTo:      c.Query("date_to"),
		Limit:       queryInt(c, "limit"),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OKWithMeta(c, nethttp.StatusOK, gin.H{"messages": messages}, meta)
}

func (h *Handler) Update(c *gin.Context) {
	var req updateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}

	message, err := h.service.Update(c.Request.Context(), messagesapp.UpdateInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		ChannelID:   c.Param("channel_id"),
		MessageID:   c.Param("message_id"),
		Body:        req.Body,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, message)
}

func (h *Handler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), messagesapp.DeleteInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		ChannelID:   c.Param("channel_id"),
		MessageID:   c.Param("message_id"),
	}); err != nil {
		slog.Warn("Xoa tin nhan that bai",
			"workspace_id", c.Param("workspace_id"),
			"channel_id", c.Param("channel_id"),
			"message_id", c.Param("message_id"),
			"actor_user_id", middleware.CurrentUserID(c),
			"error", err,
		)
		response.Error(c, err)
		return
	}
	slog.Info("Xoa tin nhan thanh cong",
		"workspace_id", c.Param("workspace_id"),
		"channel_id", c.Param("channel_id"),
		"message_id", c.Param("message_id"),
		"actor_user_id", middleware.CurrentUserID(c),
	)
	response.NoContent(c)
}

func (h *Handler) ListPins(c *gin.Context) {
	messages, err := h.service.ListPins(c.Request.Context(), messagesapp.ListPinsInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		ChannelID:   c.Param("channel_id"),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"messages": messages})
}

func (h *Handler) Pin(c *gin.Context) {
	message, err := h.service.Pin(c.Request.Context(), messagesapp.PinInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		ChannelID:   c.Param("channel_id"),
		MessageID:   c.Param("message_id"),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, message)
}

func (h *Handler) Unpin(c *gin.Context) {
	if err := h.service.Unpin(c.Request.Context(), messagesapp.PinInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		ChannelID:   c.Param("channel_id"),
		MessageID:   c.Param("message_id"),
	}); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func (h *Handler) AddReaction(c *gin.Context) {
	var req reactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}

	message, err := h.service.AddReaction(c.Request.Context(), messagesapp.ReactionInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		ChannelID:   c.Param("channel_id"),
		MessageID:   c.Param("message_id"),
		Emoji:       req.Emoji,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, message)
}

func (h *Handler) RemoveReaction(c *gin.Context) {
	emoji, err := url.PathUnescape(c.Param("emoji"))
	if err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "VALIDATION_ERROR", "Reaction không hợp lệ.", nil)
		return
	}

	message, err := h.service.RemoveReaction(c.Request.Context(), messagesapp.ReactionInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		ChannelID:   c.Param("channel_id"),
		MessageID:   c.Param("message_id"),
		Emoji:       emoji,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, message)
}

func queryInt(c *gin.Context, key string) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return 0
	}
	return value
}
