package http

import (
	"encoding/json"
	nethttp "net/http"
	"strconv"
	"strings"

	messagesapp "github.com/duclamdev/application-chat/backend/internal/modules/messages/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type scheduleMessageRequest struct {
	ParentID         string          `json:"parent_id"`
	ClientMessageID  string          `json:"client_message_id"`
	Kind             string          `json:"kind"`
	Body             string          `json:"body"`
	Metadata         json.RawMessage `json:"metadata"`
	MentionedUserIDs []string        `json:"mentioned_user_ids"`
	ScheduledFor     string          `json:"scheduled_for"`
	Silent           bool            `json:"silent"`
}

type createReminderRequest struct {
	RemindAt string `json:"remind_at"`
	Note     string `json:"note"`
}

type upsertThreadDetailsRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type threadSubscriptionRequest struct {
	Subscribed bool `json:"subscribed"`
}

type threadReadRequest struct {
	LastReadMessageID string `json:"last_read_message_id"`
}

func (h *Handler) ScheduleMessage(c *gin.Context) {
	var req scheduleMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	message, err := h.service.ScheduleMessage(c.Request.Context(), messagesapp.ScheduleMessageInput{
		ActorUserID:      middleware.CurrentUserID(c),
		WorkspaceID:      c.Param("workspace_id"),
		ChannelID:        strings.TrimSpace(c.Query("channel_id")),
		ParentID:         req.ParentID,
		ClientMessageID:  firstNonEmpty(req.ClientMessageID, c.GetHeader("Idempotency-Key")),
		Kind:             req.Kind,
		Body:             req.Body,
		Metadata:         req.Metadata,
		MentionedUserIDs: req.MentionedUserIDs,
		ScheduledFor:     req.ScheduledFor,
		Silent:           req.Silent,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, message)
}

func (h *Handler) ListScheduledMessages(c *gin.Context) {
	messages, err := h.service.ListScheduledMessages(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Query("channel_id"),
		queryIntValue(c.Query("limit")),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"scheduled_messages": messages})
}

func (h *Handler) CancelScheduledMessage(c *gin.Context) {
	if err := h.service.CancelScheduledMessage(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Param("scheduled_message_id"),
	); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(nethttp.StatusNoContent)
}

func (h *Handler) CreateReminder(c *gin.Context) {
	var req createReminderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	reminder, err := h.service.CreateReminder(c.Request.Context(), messagesapp.CreateReminderInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		ChannelID:   c.Param("channel_id"),
		MessageID:   c.Param("message_id"),
		RemindAt:    req.RemindAt,
		Note:        req.Note,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, reminder)
}

func (h *Handler) ListReminders(c *gin.Context) {
	reminders, err := h.service.ListReminders(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Query("channel_id"),
		queryIntValue(c.Query("limit")),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"reminders": reminders})
}

func (h *Handler) CancelReminder(c *gin.Context) {
	if err := h.service.CancelReminder(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Param("reminder_id"),
	); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(nethttp.StatusNoContent)
}

func (h *Handler) GetThreadDetails(c *gin.Context) {
	details, err := h.service.GetThreadDetails(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Param("channel_id"),
		c.Param("message_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, details)
}

func (h *Handler) UpsertThreadDetails(c *gin.Context) {
	var req upsertThreadDetailsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	details, err := h.service.UpsertThreadDetails(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Param("channel_id"),
		c.Param("message_id"),
		req.Title,
		req.Description,
		req.Status,
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, details)
}

func (h *Handler) SetThreadSubscription(c *gin.Context) {
	var req threadSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	details, err := h.service.SetThreadSubscription(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Param("channel_id"),
		c.Param("message_id"),
		req.Subscribed,
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, details)
}

func (h *Handler) MarkThreadRead(c *gin.Context) {
	var req threadReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	details, err := h.service.MarkThreadRead(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Param("channel_id"),
		c.Param("message_id"),
		req.LastReadMessageID,
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, details)
}

func (h *Handler) ListThreadDetails(c *gin.Context) {
	threads, err := h.service.ListThreadDetails(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Query("channel_id"),
		queryBoolValue(c.Query("subscribed_only")),
		queryIntValue(c.Query("limit")),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"threads": threads})
}

func queryIntValue(value string) int {
	result, _ := strconv.Atoi(strings.TrimSpace(value))
	return result
}

func queryBoolValue(value string) bool {
	result, _ := strconv.ParseBool(strings.TrimSpace(value))
	return result
}
