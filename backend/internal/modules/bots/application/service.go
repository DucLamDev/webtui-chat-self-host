package application

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"time"

	botsdomain "github.com/duclamdev/application-chat/backend/internal/modules/bots/domain"
	"github.com/duclamdev/application-chat/backend/internal/shared/botauto"
	"github.com/duclamdev/application-chat/backend/internal/shared/botsecrets"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

var botSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)
var botAIEnvironmentReference = regexp.MustCompile(`^env://BOT_AI_[A-Z0-9_]+$`)

type PermissionChecker interface {
	HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error)
}

type FlowRuntime interface {
	Complete(
		ctx context.Context,
		config botsdomain.AIConfig,
		flow botsdomain.Flow,
		input botauto.MessageInput,
	) (string, error)
}

type Repository interface {
	CreateBot(ctx context.Context, params CreateBotParams) (botsdomain.Bot, error)
	ListBots(ctx context.Context, workspaceID string) ([]botsdomain.Bot, error)
	InstallBot(ctx context.Context, params InstallBotParams) (botsdomain.Installation, error)
	ListInstallations(ctx context.Context, workspaceID string, botID string) ([]botsdomain.Installation, error)
	SendBotMessage(ctx context.Context, params SendBotMessageParams) (botsdomain.BotMessage, error)
	GetAIConfig(ctx context.Context, workspaceID string, botID string) (botsdomain.AIConfig, error)
	UpsertAIConfig(ctx context.Context, params AIConfigParams) (botsdomain.AIConfig, error)
	ListFlows(ctx context.Context, workspaceID string, botID string) ([]botsdomain.Flow, error)
	CreateFlow(ctx context.Context, params FlowParams) (botsdomain.Flow, error)
	UpdateFlow(ctx context.Context, params FlowParams) (botsdomain.Flow, error)
	PublishFlow(ctx context.Context, params PublishFlowParams) (botsdomain.Flow, error)
	TestFlow(ctx context.Context, params TestFlowParams) (botsdomain.FlowRun, error)
}

type Service struct {
	repo        Repository
	checker     PermissionChecker
	flowRuntime FlowRuntime
	secretKey   string
}

type CreateBotInput struct {
	ActorUserID string
	WorkspaceID string
	Slug        string
	Name        string
	Description string
	AvatarURL   string
	Settings    json.RawMessage
}

type CreateBotParams struct {
	WorkspaceID string
	Slug        string
	Name        string
	Description string
	AvatarURL   string
	CreatedBy   string
	Settings    []byte
}

type InstallBotInput struct {
	ActorUserID string
	WorkspaceID string
	BotID       string
	ChannelID   string
	Config      json.RawMessage
}

type InstallBotParams struct {
	WorkspaceID string
	BotID       string
	ChannelID   string
	Config      []byte
}

type SendBotMessageInput struct {
	ActorUserID string
	WorkspaceID string
	BotID       string
	ChannelID   string
	Body        string
	Metadata    json.RawMessage
}

type SendBotMessageParams struct {
	WorkspaceID string
	BotID       string
	ChannelID   string
	Body        string
	Metadata    []byte
}

type AIConfigInput struct {
	ActorUserID string
	WorkspaceID string
	BotID       string
	Provider    string
	Model       string
	SecretRef   string
	APIKey      string
	Settings    json.RawMessage
}

type AIConfigParams struct {
	WorkspaceID string
	BotID       string
	Provider    string
	Model       string
	SecretRef   string
	Settings    []byte
	ActorUserID string
}

type FlowInput struct {
	ActorUserID     string
	WorkspaceID     string
	BotID           string
	FlowID          string
	Name            string
	Prompt          string
	TriggerConfig   json.RawMessage
	ToolConfig      json.RawMessage
	KnowledgeConfig json.RawMessage
}

type FlowParams struct {
	WorkspaceID     string
	BotID           string
	FlowID          string
	Name            string
	Prompt          string
	TriggerConfig   []byte
	ToolConfig      []byte
	KnowledgeConfig []byte
	ActorUserID     string
}

type PublishFlowInput struct {
	ActorUserID string
	WorkspaceID string
	BotID       string
	FlowID      string
}

type PublishFlowParams struct {
	WorkspaceID string
	BotID       string
	FlowID      string
	ActorUserID string
}

type TestFlowInput struct {
	ActorUserID string
	WorkspaceID string
	BotID       string
	FlowID      string
	Input       json.RawMessage
}

type TestFlowParams struct {
	WorkspaceID string
	BotID       string
	FlowID      string
	Input       []byte
	Transcript  []byte
	Status      string
	Error       string
	ActorUserID string
}

type BotDTO struct {
	ID          string          `json:"id"`
	WorkspaceID string          `json:"workspace_id"`
	Slug        string          `json:"slug"`
	Name        string          `json:"name"`
	Description *string         `json:"description,omitempty"`
	AvatarURL   *string         `json:"avatar_url,omitempty"`
	Status      string          `json:"status"`
	CreatedBy   *string         `json:"created_by,omitempty"`
	Settings    json.RawMessage `json:"settings"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

type InstallationDTO struct {
	ID          string          `json:"id"`
	BotID       string          `json:"bot_id"`
	WorkspaceID string          `json:"workspace_id"`
	ChannelID   *string         `json:"channel_id,omitempty"`
	Status      string          `json:"status"`
	Config      json.RawMessage `json:"config"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

type BotMessageDTO struct {
	ID          string          `json:"id"`
	WorkspaceID string          `json:"workspace_id"`
	ChannelID   string          `json:"channel_id"`
	BotID       string          `json:"bot_id"`
	Kind        string          `json:"kind"`
	Body        string          `json:"body"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedAt   string          `json:"created_at"`
}

type AIConfigDTO struct {
	WorkspaceID string          `json:"workspace_id"`
	BotID       string          `json:"bot_id"`
	Provider    string          `json:"provider"`
	Model       string          `json:"model"`
	SecretRef   *string         `json:"secret_ref,omitempty"`
	Settings    json.RawMessage `json:"settings"`
	CreatedBy   *string         `json:"created_by,omitempty"`
	UpdatedBy   *string         `json:"updated_by,omitempty"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

type FlowDTO struct {
	ID              string          `json:"id"`
	WorkspaceID     string          `json:"workspace_id"`
	BotID           string          `json:"bot_id"`
	Version         int             `json:"version"`
	Status          string          `json:"status"`
	Name            string          `json:"name"`
	Prompt          string          `json:"prompt"`
	TriggerConfig   json.RawMessage `json:"trigger_config"`
	ToolConfig      json.RawMessage `json:"tool_config"`
	KnowledgeConfig json.RawMessage `json:"knowledge_config"`
	CreatedBy       *string         `json:"created_by,omitempty"`
	UpdatedBy       *string         `json:"updated_by,omitempty"`
	PublishedAt     *string         `json:"published_at,omitempty"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
}

type FlowRunDTO struct {
	ID          string          `json:"id"`
	WorkspaceID string          `json:"workspace_id"`
	BotID       string          `json:"bot_id"`
	FlowID      string          `json:"flow_id"`
	Input       json.RawMessage `json:"input"`
	Transcript  json.RawMessage `json:"transcript"`
	Status      string          `json:"status"`
	Error       *string         `json:"error,omitempty"`
	CreatedBy   *string         `json:"created_by,omitempty"`
	CreatedAt   string          `json:"created_at"`
}

func NewService(repo Repository, checker PermissionChecker) *Service {
	return &Service{repo: repo, checker: checker}
}

func (s *Service) SetFlowRuntime(runtime FlowRuntime) {
	s.flowRuntime = runtime
}

func (s *Service) SetSecretMasterKey(secret string) {
	s.secretKey = strings.TrimSpace(secret)
}

// HandleMessage lets every published bot flow act as a workspace-scoped
// responder. A bot only receives messages from channels where it is installed,
// and the flow trigger decides whether the model should run.
func (s *Service) HandleMessage(ctx context.Context, input botauto.MessageInput) ([]botauto.BotMessage, error) {
	if s == nil || s.repo == nil || s.flowRuntime == nil {
		return nil, nil
	}
	workspaceID := strings.TrimSpace(input.WorkspaceID)
	channelID := strings.TrimSpace(input.ChannelID)
	if workspaceID == "" || channelID == "" || strings.TrimSpace(input.Body) == "" {
		return nil, nil
	}

	bots, err := s.repo.ListBots(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	responses := make([]botauto.BotMessage, 0)
	for _, bot := range bots {
		if bot.Status != "active" {
			continue
		}
		installations, listErr := s.repo.ListInstallations(ctx, workspaceID, bot.ID)
		if listErr != nil || !botInstalledInChannel(installations, channelID) {
			continue
		}
		flows, listErr := s.repo.ListFlows(ctx, workspaceID, bot.ID)
		if listErr != nil {
			continue
		}
		for _, flow := range flows {
			if flow.Status != "published" || !botFlowMatches(flow, bot, input.Body) {
				continue
			}
			config, configErr := s.repo.GetAIConfig(ctx, workspaceID, bot.ID)
			if configErr != nil {
				slog.Warn("Bot chưa có cấu hình AI khả dụng",
					"workspace_id", workspaceID,
					"channel_id", channelID,
					"bot_id", bot.ID,
					"error", configErr,
				)
				break
			}
			body, completionErr := s.flowRuntime.Complete(ctx, config, flow, input)
			if completionErr != nil {
				slog.Warn("Bot không tạo được phản hồi AI",
					"workspace_id", workspaceID,
					"channel_id", channelID,
					"bot_id", bot.ID,
					"flow_id", flow.ID,
					"error", completionErr,
				)
				break
			}
			body = strings.TrimSpace(body)
			if body == "" {
				break
			}
			metadata, _ := json.Marshal(map[string]any{
				"bot_name":          bot.Name,
				"bot_slug":          bot.Slug,
				"flow_id":           flow.ID,
				"flow_name":         flow.Name,
				"source":            "generic_bot_flow",
				"source_message_id": input.MessageID,
			})
			message, sendErr := s.repo.SendBotMessage(ctx, SendBotMessageParams{
				WorkspaceID: workspaceID,
				BotID:       bot.ID,
				ChannelID:   channelID,
				Body:        body,
				Metadata:    metadata,
			})
			if sendErr != nil {
				return responses, sendErr
			}
			responses = append(responses, botauto.BotMessage{
				ID:          message.ID,
				WorkspaceID: message.WorkspaceID,
				ChannelID:   message.ChannelID,
				BotID:       message.BotID,
				Kind:        message.Kind,
				Body:        message.Body,
				Metadata:    message.Metadata,
				CreatedAt:   message.CreatedAt.UTC().Format(time.RFC3339Nano),
			})
			break
		}
	}
	return responses, nil
}

func botInstalledInChannel(installations []botsdomain.Installation, channelID string) bool {
	for _, installation := range installations {
		if installation.Status != "active" {
			continue
		}
		if installation.ChannelID == nil || strings.TrimSpace(*installation.ChannelID) == channelID {
			return true
		}
	}
	return false
}

func botFlowMatches(flow botsdomain.Flow, bot botsdomain.Bot, body string) bool {
	var trigger struct {
		Type     string   `json:"type"`
		Keywords []string `json:"keywords"`
		Prefix   string   `json:"prefix"`
	}
	if len(flow.TriggerConfig) > 0 {
		_ = json.Unmarshal(flow.TriggerConfig, &trigger)
	}
	normalizedBody := strings.ToLower(strings.TrimSpace(body))
	switch strings.ToLower(strings.TrimSpace(trigger.Type)) {
	case "", "mention":
		return strings.Contains(normalizedBody, "@"+strings.ToLower(bot.Slug)) ||
			strings.Contains(normalizedBody, "@"+strings.ToLower(bot.Name))
	case "all", "always", "message":
		return true
	case "command":
		prefix := strings.ToLower(strings.TrimSpace(trigger.Prefix))
		return prefix != "" && strings.HasPrefix(normalizedBody, prefix)
	case "keyword", "keywords":
		for _, keyword := range trigger.Keywords {
			if keyword = strings.ToLower(strings.TrimSpace(keyword)); keyword != "" &&
				strings.Contains(normalizedBody, keyword) {
				return true
			}
		}
	}
	return false
}

func (s *Service) CreateBot(ctx context.Context, input CreateBotInput) (BotDTO, error) {
	if err := s.ensureManagePermission(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return BotDTO{}, err
	}
	slug := strings.ToLower(strings.TrimSpace(input.Slug))
	name := strings.TrimSpace(input.Name)
	if !botSlugPattern.MatchString(slug) {
		return BotDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Slug bot phải dài 3-63 ký tự và chỉ gồm chữ thường, số hoặc dấu gạch ngang.")
	}
	if name == "" || len([]rune(name)) > 120 {
		return BotDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Tên bot phải dài từ 1 đến 120 ký tự.")
	}
	settings, err := normalizeJSON(input.Settings, "Settings bot không phải JSON hợp lệ.")
	if err != nil {
		return BotDTO{}, err
	}
	bot, err := s.repo.CreateBot(ctx, CreateBotParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		Slug:        slug,
		Name:        name,
		Description: strings.TrimSpace(input.Description),
		AvatarURL:   strings.TrimSpace(input.AvatarURL),
		CreatedBy:   strings.TrimSpace(input.ActorUserID),
		Settings:    settings,
	})
	if err != nil {
		return BotDTO{}, mapBotError(err)
	}
	return toBotDTO(bot), nil
}

func (s *Service) ListBots(ctx context.Context, actorUserID string, workspaceID string) ([]BotDTO, error) {
	if err := s.ensureManagePermission(ctx, actorUserID, workspaceID); err != nil {
		return nil, err
	}
	bots, err := s.repo.ListBots(ctx, strings.TrimSpace(workspaceID))
	if err != nil {
		return nil, err
	}
	return toBotDTOs(bots), nil
}

func (s *Service) InstallBot(ctx context.Context, input InstallBotInput) (InstallationDTO, error) {
	if err := s.ensureManagePermission(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return InstallationDTO{}, err
	}
	config, err := normalizeJSON(input.Config, "Config cài đặt bot không phải JSON hợp lệ.")
	if err != nil {
		return InstallationDTO{}, err
	}
	installation, err := s.repo.InstallBot(ctx, InstallBotParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		BotID:       strings.TrimSpace(input.BotID),
		ChannelID:   strings.TrimSpace(input.ChannelID),
		Config:      config,
	})
	if err != nil {
		return InstallationDTO{}, mapBotError(err)
	}
	return toInstallationDTO(installation), nil
}

func (s *Service) ListInstallations(ctx context.Context, actorUserID string, workspaceID string, botID string) ([]InstallationDTO, error) {
	if err := s.ensureManagePermission(ctx, actorUserID, workspaceID); err != nil {
		return nil, err
	}
	installations, err := s.repo.ListInstallations(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(botID))
	if err != nil {
		return nil, err
	}
	return toInstallationDTOs(installations), nil
}

func (s *Service) SendMessage(ctx context.Context, input SendBotMessageInput) (BotMessageDTO, error) {
	if err := s.ensureManagePermission(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return BotMessageDTO{}, err
	}
	body := strings.TrimSpace(input.Body)
	if body == "" || len([]rune(body)) > 8000 {
		return BotMessageDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Nội dung bot message phải dài từ 1 đến 8000 ký tự.")
	}
	metadata, err := normalizeJSON(input.Metadata, "Metadata bot message không phải JSON hợp lệ.")
	if err != nil {
		return BotMessageDTO{}, err
	}
	message, err := s.repo.SendBotMessage(ctx, SendBotMessageParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		BotID:       strings.TrimSpace(input.BotID),
		ChannelID:   strings.TrimSpace(input.ChannelID),
		Body:        body,
		Metadata:    metadata,
	})
	if err != nil {
		return BotMessageDTO{}, mapBotError(err)
	}
	return toBotMessageDTO(message), nil
}

func (s *Service) GetAIConfig(ctx context.Context, actorUserID string, workspaceID string, botID string) (AIConfigDTO, error) {
	if err := s.ensureManagePermission(ctx, actorUserID, workspaceID); err != nil {
		return AIConfigDTO{}, err
	}
	config, err := s.repo.GetAIConfig(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(botID))
	if err != nil {
		return AIConfigDTO{}, mapBotError(err)
	}
	return toAIConfigDTO(config), nil
}

func (s *Service) UpsertAIConfig(ctx context.Context, input AIConfigInput) (AIConfigDTO, error) {
	if err := s.ensureManagePermission(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return AIConfigDTO{}, err
	}
	provider := strings.TrimSpace(input.Provider)
	model := strings.TrimSpace(input.Model)
	if provider == "" || model == "" {
		return AIConfigDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Provider và model AI là bắt buộc.")
	}
	settings, err := normalizeJSON(input.Settings, "Settings AI không phải JSON hợp lệ.")
	if err != nil {
		return AIConfigDTO{}, err
	}
	secretRef := strings.TrimSpace(input.SecretRef)
	apiKey := strings.TrimSpace(input.APIKey)
	if len(apiKey) > 4096 {
		return AIConfigDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "API key không được vượt quá 4096 ký tự.")
	}
	if apiKey != "" {
		if s.secretKey == "" {
			return AIConfigDTO{}, apperrors.ServiceUnavailable("BOT_SECRET_STORAGE_UNAVAILABLE", "Máy chủ chưa cấu hình khóa mã hóa API key cho bot.")
		}
		secretRef, err = botsecrets.Encrypt(s.secretKey, apiKey)
		if err != nil {
			return AIConfigDTO{}, apperrors.Internal("Không mã hóa được API key của bot.")
		}
	} else if botsecrets.IsMasked(secretRef) {
		existing, getErr := s.repo.GetAIConfig(ctx, strings.TrimSpace(input.WorkspaceID), strings.TrimSpace(input.BotID))
		if getErr != nil || existing.SecretRef == nil || !botsecrets.IsEncrypted(*existing.SecretRef) {
			return AIConfigDTO{}, apperrors.BadRequest("BOT_SECRET_NOT_FOUND", "API key đã lưu không còn tồn tại. Hãy nhập lại API key.")
		}
		secretRef = *existing.SecretRef
	} else if secretRef != "" && !botAIEnvironmentReference.MatchString(secretRef) {
		return AIConfigDTO{}, apperrors.BadRequest("INVALID_SECRET_REF", "Secret reference phải có dạng env://BOT_AI_*.")
	}
	config, err := s.repo.UpsertAIConfig(ctx, AIConfigParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		BotID:       strings.TrimSpace(input.BotID),
		Provider:    provider,
		Model:       model,
		SecretRef:   secretRef,
		Settings:    settings,
		ActorUserID: strings.TrimSpace(input.ActorUserID),
	})
	if err != nil {
		return AIConfigDTO{}, mapBotError(err)
	}
	return toAIConfigDTO(config), nil
}

func (s *Service) ListFlows(ctx context.Context, actorUserID string, workspaceID string, botID string) ([]FlowDTO, error) {
	if err := s.ensureManagePermission(ctx, actorUserID, workspaceID); err != nil {
		return nil, err
	}
	flows, err := s.repo.ListFlows(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(botID))
	if err != nil {
		return nil, mapBotError(err)
	}
	return toFlowDTOs(flows), nil
}

func (s *Service) CreateFlow(ctx context.Context, input FlowInput) (FlowDTO, error) {
	params, err := s.normalizeFlowInput(ctx, input)
	if err != nil {
		return FlowDTO{}, err
	}
	flow, err := s.repo.CreateFlow(ctx, params)
	if err != nil {
		return FlowDTO{}, mapBotError(err)
	}
	return toFlowDTO(flow), nil
}

func (s *Service) UpdateFlow(ctx context.Context, input FlowInput) (FlowDTO, error) {
	params, err := s.normalizeFlowInput(ctx, input)
	if err != nil {
		return FlowDTO{}, err
	}
	params.FlowID = strings.TrimSpace(input.FlowID)
	if params.FlowID == "" {
		return FlowDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "flow_id là bắt buộc.")
	}
	flow, err := s.repo.UpdateFlow(ctx, params)
	if err != nil {
		return FlowDTO{}, mapBotError(err)
	}
	return toFlowDTO(flow), nil
}

func (s *Service) PublishFlow(ctx context.Context, input PublishFlowInput) (FlowDTO, error) {
	if err := s.ensureManagePermission(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return FlowDTO{}, err
	}
	flow, err := s.repo.PublishFlow(ctx, PublishFlowParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		BotID:       strings.TrimSpace(input.BotID),
		FlowID:      strings.TrimSpace(input.FlowID),
		ActorUserID: strings.TrimSpace(input.ActorUserID),
	})
	if err != nil {
		return FlowDTO{}, mapBotError(err)
	}
	return toFlowDTO(flow), nil
}

func (s *Service) TestFlow(ctx context.Context, input TestFlowInput) (FlowRunDTO, error) {
	if err := s.ensureManagePermission(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return FlowRunDTO{}, err
	}
	payload, err := normalizeJSON(input.Input, "Input test bot không phải JSON hợp lệ.")
	if err != nil {
		return FlowRunDTO{}, err
	}
	if s.flowRuntime == nil {
		return FlowRunDTO{}, apperrors.ServiceUnavailable("BOT_AI_RUNTIME_UNAVAILABLE", "Runtime AI của bot chưa sẵn sàng.")
	}
	workspaceID := strings.TrimSpace(input.WorkspaceID)
	botID := strings.TrimSpace(input.BotID)
	flowID := strings.TrimSpace(input.FlowID)
	config, err := s.repo.GetAIConfig(ctx, workspaceID, botID)
	if err != nil {
		return FlowRunDTO{}, mapBotError(err)
	}
	flows, err := s.repo.ListFlows(ctx, workspaceID, botID)
	if err != nil {
		return FlowRunDTO{}, mapBotError(err)
	}
	var selectedFlow *botsdomain.Flow
	for index := range flows {
		if flows[index].ID == flowID {
			selectedFlow = &flows[index]
			break
		}
	}
	if selectedFlow == nil {
		return FlowRunDTO{}, apperrors.NotFound("BOT_FLOW_NOT_FOUND", "Không tìm thấy nghiệp vụ bot.")
	}
	messageBody := string(payload)
	var testInput struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(payload, &testInput) == nil && strings.TrimSpace(testInput.Message) != "" {
		messageBody = strings.TrimSpace(testInput.Message)
	}
	completion, completionErr := s.flowRuntime.Complete(ctx, config, *selectedFlow, botauto.MessageInput{
		ActorUserID: strings.TrimSpace(input.ActorUserID),
		WorkspaceID: workspaceID,
		Body:        messageBody,
	})
	status := "success"
	errorMessage := ""
	transcriptValue := map[string]any{"content": strings.TrimSpace(completion)}
	if completionErr != nil {
		status = "failed"
		errorMessage = completionErr.Error()
		transcriptValue = map[string]any{"error": errorMessage}
	}
	transcript, marshalErr := json.Marshal(transcriptValue)
	if marshalErr != nil {
		return FlowRunDTO{}, apperrors.Internal("Không ghi được kết quả chạy thử bot.")
	}
	run, err := s.repo.TestFlow(ctx, TestFlowParams{
		WorkspaceID: workspaceID,
		BotID:       botID,
		FlowID:      flowID,
		Input:       payload,
		Transcript:  transcript,
		Status:      status,
		Error:       errorMessage,
		ActorUserID: strings.TrimSpace(input.ActorUserID),
	})
	if err != nil {
		return FlowRunDTO{}, mapBotError(err)
	}
	return toFlowRunDTO(run), nil
}

func (s *Service) normalizeFlowInput(ctx context.Context, input FlowInput) (FlowParams, error) {
	if err := s.ensureManagePermission(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return FlowParams{}, err
	}
	name := strings.TrimSpace(input.Name)
	prompt := strings.TrimSpace(input.Prompt)
	if name == "" || len([]rune(name)) > 120 {
		return FlowParams{}, apperrors.BadRequest("VALIDATION_ERROR", "Tên flow phải dài từ 1 đến 120 ký tự.")
	}
	if prompt == "" || len([]rune(prompt)) > 12000 {
		return FlowParams{}, apperrors.BadRequest("VALIDATION_ERROR", "Prompt flow phải dài từ 1 đến 12000 ký tự.")
	}
	triggerConfig, err := normalizeJSON(input.TriggerConfig, "Trigger config không phải JSON hợp lệ.")
	if err != nil {
		return FlowParams{}, err
	}
	toolConfig, err := normalizeJSON(input.ToolConfig, "Tool config không phải JSON hợp lệ.")
	if err != nil {
		return FlowParams{}, err
	}
	knowledgeConfig, err := normalizeJSON(input.KnowledgeConfig, "Knowledge config không phải JSON hợp lệ.")
	if err != nil {
		return FlowParams{}, err
	}
	return FlowParams{
		WorkspaceID:     strings.TrimSpace(input.WorkspaceID),
		BotID:           strings.TrimSpace(input.BotID),
		Name:            name,
		Prompt:          prompt,
		TriggerConfig:   triggerConfig,
		ToolConfig:      toolConfig,
		KnowledgeConfig: knowledgeConfig,
		ActorUserID:     strings.TrimSpace(input.ActorUserID),
	}, nil
}

func (s *Service) ensureManagePermission(ctx context.Context, userID string, workspaceID string) error {
	allowed, err := s.checker.HasWorkspacePermission(ctx, strings.TrimSpace(userID), strings.TrimSpace(workspaceID), "bot.manage")
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("Bạn không có quyền quản lý bot.")
	}
	return nil
}

func normalizeJSON(value json.RawMessage, message string) ([]byte, error) {
	if len(value) == 0 || strings.TrimSpace(string(value)) == "" || strings.TrimSpace(string(value)) == "null" {
		return []byte(`{}`), nil
	}
	if !json.Valid(value) {
		return nil, apperrors.BadRequest("VALIDATION_ERROR", message)
	}
	return []byte(value), nil
}

func mapBotError(err error) error {
	if errors.Is(err, botsdomain.ErrBotNotFound) {
		return apperrors.NotFound("BOT_NOT_FOUND", "Không tìm thấy bot.")
	}
	if errors.Is(err, botsdomain.ErrBotAlreadyExists) {
		return apperrors.Conflict("BOT_ALREADY_EXISTS", "Bot đã tồn tại trong workspace.")
	}
	if errors.Is(err, botsdomain.ErrBotAlreadyInstalled) {
		return apperrors.Conflict("BOT_ALREADY_INSTALLED", "Bot đã được cài đặt.")
	}
	if errors.Is(err, botsdomain.ErrBotNotInstalled) {
		return apperrors.BadRequest("BOT_NOT_INSTALLED", "Bot chưa được cài đặt vào kênh.")
	}
	return err
}

func toBotDTOs(bots []botsdomain.Bot) []BotDTO {
	dtos := make([]BotDTO, 0, len(bots))
	for _, bot := range bots {
		dtos = append(dtos, toBotDTO(bot))
	}
	return dtos
}

func toBotDTO(bot botsdomain.Bot) BotDTO {
	settings := json.RawMessage(bot.Settings)
	if len(settings) == 0 {
		settings = json.RawMessage(`{}`)
	}
	return BotDTO{
		ID:          bot.ID,
		WorkspaceID: bot.WorkspaceID,
		Slug:        bot.Slug,
		Name:        bot.Name,
		Description: bot.Description,
		AvatarURL:   bot.AvatarURL,
		Status:      bot.Status,
		CreatedBy:   bot.CreatedBy,
		Settings:    settings,
		CreatedAt:   formatTime(bot.CreatedAt),
		UpdatedAt:   formatTime(bot.UpdatedAt),
	}
}

func toInstallationDTOs(installations []botsdomain.Installation) []InstallationDTO {
	dtos := make([]InstallationDTO, 0, len(installations))
	for _, installation := range installations {
		dtos = append(dtos, toInstallationDTO(installation))
	}
	return dtos
}

func toInstallationDTO(installation botsdomain.Installation) InstallationDTO {
	config := json.RawMessage(installation.Config)
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}
	return InstallationDTO{
		ID:          installation.ID,
		BotID:       installation.BotID,
		WorkspaceID: installation.WorkspaceID,
		ChannelID:   installation.ChannelID,
		Status:      installation.Status,
		Config:      config,
		CreatedAt:   formatTime(installation.CreatedAt),
		UpdatedAt:   formatTime(installation.UpdatedAt),
	}
}

func toBotMessageDTO(message botsdomain.BotMessage) BotMessageDTO {
	metadata := json.RawMessage(message.Metadata)
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	return BotMessageDTO{
		ID:          message.ID,
		WorkspaceID: message.WorkspaceID,
		ChannelID:   message.ChannelID,
		BotID:       message.BotID,
		Kind:        message.Kind,
		Body:        message.Body,
		Metadata:    metadata,
		CreatedAt:   formatTime(message.CreatedAt),
	}
}

func toAIConfigDTO(config botsdomain.AIConfig) AIConfigDTO {
	settings := json.RawMessage(config.Settings)
	if len(settings) == 0 {
		settings = json.RawMessage(`{}`)
	}
	secretRef := config.SecretRef
	if secretRef != nil && botsecrets.IsEncrypted(*secretRef) {
		masked := botsecrets.MaskedReference()
		secretRef = &masked
	}
	return AIConfigDTO{
		WorkspaceID: config.WorkspaceID,
		BotID:       config.BotID,
		Provider:    config.Provider,
		Model:       config.Model,
		SecretRef:   secretRef,
		Settings:    settings,
		CreatedBy:   config.CreatedBy,
		UpdatedBy:   config.UpdatedBy,
		CreatedAt:   formatTime(config.CreatedAt),
		UpdatedAt:   formatTime(config.UpdatedAt),
	}
}

func toFlowDTOs(flows []botsdomain.Flow) []FlowDTO {
	dtos := make([]FlowDTO, 0, len(flows))
	for _, flow := range flows {
		dtos = append(dtos, toFlowDTO(flow))
	}
	return dtos
}

func toFlowDTO(flow botsdomain.Flow) FlowDTO {
	triggerConfig := json.RawMessage(flow.TriggerConfig)
	if len(triggerConfig) == 0 {
		triggerConfig = json.RawMessage(`{}`)
	}
	toolConfig := json.RawMessage(flow.ToolConfig)
	if len(toolConfig) == 0 {
		toolConfig = json.RawMessage(`{}`)
	}
	knowledgeConfig := json.RawMessage(flow.KnowledgeConfig)
	if len(knowledgeConfig) == 0 {
		knowledgeConfig = json.RawMessage(`{}`)
	}
	return FlowDTO{
		ID:              flow.ID,
		WorkspaceID:     flow.WorkspaceID,
		BotID:           flow.BotID,
		Version:         flow.Version,
		Status:          flow.Status,
		Name:            flow.Name,
		Prompt:          flow.Prompt,
		TriggerConfig:   triggerConfig,
		ToolConfig:      toolConfig,
		KnowledgeConfig: knowledgeConfig,
		CreatedBy:       flow.CreatedBy,
		UpdatedBy:       flow.UpdatedBy,
		PublishedAt:     formatTimePtr(flow.PublishedAt),
		CreatedAt:       formatTime(flow.CreatedAt),
		UpdatedAt:       formatTime(flow.UpdatedAt),
	}
}

func toFlowRunDTO(run botsdomain.FlowRun) FlowRunDTO {
	input := json.RawMessage(run.Input)
	if len(input) == 0 {
		input = json.RawMessage(`{}`)
	}
	transcript := json.RawMessage(run.Transcript)
	if len(transcript) == 0 {
		transcript = json.RawMessage(`{}`)
	}
	return FlowRunDTO{
		ID:          run.ID,
		WorkspaceID: run.WorkspaceID,
		BotID:       run.BotID,
		FlowID:      run.FlowID,
		Input:       input,
		Transcript:  transcript,
		Status:      run.Status,
		Error:       run.Error,
		CreatedBy:   run.CreatedBy,
		CreatedAt:   formatTime(run.CreatedAt),
	}
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}

func formatTimePtr(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := formatTime(*value)
	return &formatted
}
