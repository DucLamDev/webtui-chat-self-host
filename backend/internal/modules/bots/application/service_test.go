package application

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	botsdomain "github.com/duclamdev/application-chat/backend/internal/modules/bots/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type fakeBotChecker struct {
	allowed bool
}

func (c fakeBotChecker) HasWorkspacePermission(context.Context, string, string, string) (bool, error) {
	return c.allowed, nil
}

type fakeBotRepo struct {
	createCalled   bool
	upsertAICalled bool
	createFlowCall bool
}

func (r *fakeBotRepo) CreateBot(context.Context, CreateBotParams) (botsdomain.Bot, error) {
	r.createCalled = true
	return botsdomain.Bot{}, nil
}

func (r *fakeBotRepo) ListBots(context.Context, string) ([]botsdomain.Bot, error) {
	return nil, nil
}

func (r *fakeBotRepo) InstallBot(context.Context, InstallBotParams) (botsdomain.Installation, error) {
	return botsdomain.Installation{}, nil
}

func (r *fakeBotRepo) ListInstallations(context.Context, string, string) ([]botsdomain.Installation, error) {
	return nil, nil
}

func (r *fakeBotRepo) SendBotMessage(context.Context, SendBotMessageParams) (botsdomain.BotMessage, error) {
	return botsdomain.BotMessage{}, nil
}

func (r *fakeBotRepo) GetAIConfig(context.Context, string, string) (botsdomain.AIConfig, error) {
	now := time.Now()
	return botsdomain.AIConfig{
		WorkspaceID: "workspace-1",
		BotID:       "bot-1",
		Provider:    "openai",
		Model:       "gpt-4.1-mini",
		Settings:    []byte(`{}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (r *fakeBotRepo) UpsertAIConfig(_ context.Context, params AIConfigParams) (botsdomain.AIConfig, error) {
	r.upsertAICalled = true
	now := time.Now()
	return botsdomain.AIConfig{
		WorkspaceID: params.WorkspaceID,
		BotID:       params.BotID,
		Provider:    params.Provider,
		Model:       params.Model,
		Settings:    params.Settings,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (r *fakeBotRepo) ListFlows(context.Context, string, string) ([]botsdomain.Flow, error) {
	return nil, nil
}

func (r *fakeBotRepo) CreateFlow(_ context.Context, params FlowParams) (botsdomain.Flow, error) {
	r.createFlowCall = true
	now := time.Now()
	return botsdomain.Flow{
		ID:              "flow-1",
		WorkspaceID:     params.WorkspaceID,
		BotID:           params.BotID,
		Version:         1,
		Status:          "draft",
		Name:            params.Name,
		Prompt:          params.Prompt,
		TriggerConfig:   params.TriggerConfig,
		ToolConfig:      params.ToolConfig,
		KnowledgeConfig: params.KnowledgeConfig,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

func (r *fakeBotRepo) UpdateFlow(context.Context, FlowParams) (botsdomain.Flow, error) {
	return botsdomain.Flow{}, nil
}

func (r *fakeBotRepo) PublishFlow(context.Context, PublishFlowParams) (botsdomain.Flow, error) {
	return botsdomain.Flow{}, nil
}

func (r *fakeBotRepo) TestFlow(context.Context, TestFlowParams) (botsdomain.FlowRun, error) {
	return botsdomain.FlowRun{}, nil
}

func TestCreateBotRejectsInvalidSlugBeforeRepository(t *testing.T) {
	repo := &fakeBotRepo{}
	service := NewService(repo, fakeBotChecker{allowed: true})

	_, err := service.CreateBot(context.Background(), CreateBotInput{
		ActorUserID: "user-1",
		WorkspaceID: "workspace-1",
		Slug:        "Bot X",
		Name:        "Bot X",
	})
	if err == nil {
		t.Fatal("CreateBot() phải trả lỗi slug không hợp lệ")
	}
	if repo.createCalled {
		t.Fatal("CreateBot() không được gọi repository khi validation lỗi")
	}
	if appErr, ok := err.(*apperrors.AppError); !ok || appErr.Code != "VALIDATION_ERROR" {
		t.Fatalf("lỗi = %#v, muốn VALIDATION_ERROR", err)
	}
}

func TestSendMessageRejectsEmptyBody(t *testing.T) {
	service := NewService(&fakeBotRepo{}, fakeBotChecker{allowed: true})

	_, err := service.SendMessage(context.Background(), SendBotMessageInput{
		ActorUserID: "user-1",
		WorkspaceID: "workspace-1",
		BotID:       "bot-1",
		ChannelID:   "channel-1",
		Body:        "   ",
	})
	if err == nil {
		t.Fatal("SendMessage() phải trả lỗi khi body rỗng")
	}
}

func TestUpsertAIConfigRejectsMissingProviderBeforeRepository(t *testing.T) {
	repo := &fakeBotRepo{}
	service := NewService(repo, fakeBotChecker{allowed: true})

	_, err := service.UpsertAIConfig(context.Background(), AIConfigInput{
		ActorUserID: "user-1",
		WorkspaceID: "workspace-1",
		BotID:       "bot-1",
		Model:       "gpt-4.1-mini",
	})
	if err == nil {
		t.Fatal("UpsertAIConfig() phải trả lỗi khi thiếu provider")
	}
	if repo.upsertAICalled {
		t.Fatal("UpsertAIConfig() không được gọi repository khi validation lỗi")
	}
}

func TestCreateFlowRejectsInvalidJSONBeforeRepository(t *testing.T) {
	repo := &fakeBotRepo{}
	service := NewService(repo, fakeBotChecker{allowed: true})

	_, err := service.CreateFlow(context.Background(), FlowInput{
		ActorUserID:   "user-1",
		WorkspaceID:   "workspace-1",
		BotID:         "bot-1",
		Name:          "Kiểm tra hạn dịch vụ",
		Prompt:        "Kiểm tra dịch vụ sắp hết hạn theo email.",
		TriggerConfig: json.RawMessage(`{"type":`),
	})
	if err == nil {
		t.Fatal("CreateFlow() phải trả lỗi khi trigger_config không phải JSON hợp lệ")
	}
	if repo.createFlowCall {
		t.Fatal("CreateFlow() không được gọi repository khi validation lỗi")
	}
}
