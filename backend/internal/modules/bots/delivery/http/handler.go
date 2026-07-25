package http

import (
	"encoding/json"
	nethttp "net/http"

	botsapp "github.com/duclamdev/application-chat/backend/internal/modules/bots/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *botsapp.Service
}

type createBotRequest struct {
	Slug        string          `json:"slug"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	AvatarURL   string          `json:"avatar_url"`
	Settings    json.RawMessage `json:"settings"`
}

type installBotRequest struct {
	ChannelID string          `json:"channel_id"`
	Config    json.RawMessage `json:"config"`
}

type sendBotMessageRequest struct {
	ChannelID string          `json:"channel_id"`
	Body      string          `json:"body"`
	Metadata  json.RawMessage `json:"metadata"`
}

type aiConfigRequest struct {
	Provider  string          `json:"provider"`
	Model     string          `json:"model"`
	SecretRef string          `json:"secret_ref"`
	Settings  json.RawMessage `json:"settings"`
}

type flowRequest struct {
	Name            string          `json:"name"`
	Prompt          string          `json:"prompt"`
	TriggerConfig   json.RawMessage `json:"trigger_config"`
	ToolConfig      json.RawMessage `json:"tool_config"`
	KnowledgeConfig json.RawMessage `json:"knowledge_config"`
}

type testFlowRequest struct {
	Input json.RawMessage `json:"input"`
}

func NewHandler(service *botsapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	private := router.Group("/workspaces/:workspace_id")
	private.Use(authMiddleware)
	private.GET("/bots", h.ListBots)
	private.POST("/bots", h.CreateBot)
	private.GET("/bots/:bot_id/ai-config", h.GetAIConfig)
	private.PUT("/bots/:bot_id/ai-config", h.UpsertAIConfig)
	private.GET("/bots/:bot_id/flows", h.ListFlows)
	private.POST("/bots/:bot_id/flows", h.CreateFlow)
	private.PATCH("/bots/:bot_id/flows/:flow_id", h.UpdateFlow)
	private.POST("/bots/:bot_id/flows/:flow_id/publish", h.PublishFlow)
	private.POST("/bots/:bot_id/flows/:flow_id/test", h.TestFlow)
	private.GET("/bots/:bot_id/installations", h.ListInstallations)
	private.POST("/bots/:bot_id/installations", h.InstallBot)
	private.POST("/bots/:bot_id/messages", h.SendMessage)
}

func (h *Handler) CreateBot(c *gin.Context) {
	var req createBotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	bot, err := h.service.CreateBot(c.Request.Context(), botsapp.CreateBotInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		Slug:        req.Slug,
		Name:        req.Name,
		Description: req.Description,
		AvatarURL:   req.AvatarURL,
		Settings:    req.Settings,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, bot)
}

func (h *Handler) ListBots(c *gin.Context) {
	bots, err := h.service.ListBots(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"bots": bots})
}

func (h *Handler) InstallBot(c *gin.Context) {
	var req installBotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	installation, err := h.service.InstallBot(c.Request.Context(), botsapp.InstallBotInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		BotID:       c.Param("bot_id"),
		ChannelID:   req.ChannelID,
		Config:      req.Config,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, installation)
}

func (h *Handler) ListInstallations(c *gin.Context) {
	installations, err := h.service.ListInstallations(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("bot_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"installations": installations})
}

func (h *Handler) SendMessage(c *gin.Context) {
	var req sendBotMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	message, err := h.service.SendMessage(c.Request.Context(), botsapp.SendBotMessageInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		BotID:       c.Param("bot_id"),
		ChannelID:   req.ChannelID,
		Body:        req.Body,
		Metadata:    req.Metadata,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, message)
}

func (h *Handler) GetAIConfig(c *gin.Context) {
	config, err := h.service.GetAIConfig(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("bot_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, config)
}

func (h *Handler) UpsertAIConfig(c *gin.Context) {
	var req aiConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	config, err := h.service.UpsertAIConfig(c.Request.Context(), botsapp.AIConfigInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		BotID:       c.Param("bot_id"),
		Provider:    req.Provider,
		Model:       req.Model,
		SecretRef:   req.SecretRef,
		Settings:    req.Settings,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, config)
}

func (h *Handler) ListFlows(c *gin.Context) {
	flows, err := h.service.ListFlows(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("bot_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"flows": flows})
}

func (h *Handler) CreateFlow(c *gin.Context) {
	var req flowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	flow, err := h.service.CreateFlow(c.Request.Context(), toFlowInput(c, req, ""))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, flow)
}

func (h *Handler) UpdateFlow(c *gin.Context) {
	var req flowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	flow, err := h.service.UpdateFlow(c.Request.Context(), toFlowInput(c, req, c.Param("flow_id")))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, flow)
}

func (h *Handler) PublishFlow(c *gin.Context) {
	flow, err := h.service.PublishFlow(c.Request.Context(), botsapp.PublishFlowInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		BotID:       c.Param("bot_id"),
		FlowID:      c.Param("flow_id"),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, flow)
}

func (h *Handler) TestFlow(c *gin.Context) {
	var req testFlowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	run, err := h.service.TestFlow(c.Request.Context(), botsapp.TestFlowInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		BotID:       c.Param("bot_id"),
		FlowID:      c.Param("flow_id"),
		Input:       req.Input,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, run)
}

func toFlowInput(c *gin.Context, req flowRequest, flowID string) botsapp.FlowInput {
	return botsapp.FlowInput{
		ActorUserID:     middleware.CurrentUserID(c),
		WorkspaceID:     c.Param("workspace_id"),
		BotID:           c.Param("bot_id"),
		FlowID:          flowID,
		Name:            req.Name,
		Prompt:          req.Prompt,
		TriggerConfig:   req.TriggerConfig,
		ToolConfig:      req.ToolConfig,
		KnowledgeConfig: req.KnowledgeConfig,
	}
}
