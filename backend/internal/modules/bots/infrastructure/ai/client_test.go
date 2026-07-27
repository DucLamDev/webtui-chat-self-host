package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	botsdomain "github.com/duclamdev/application-chat/backend/internal/modules/bots/domain"
	"github.com/duclamdev/application-chat/backend/internal/shared/botauto"
)

func TestCompleteCallsOllamaCompatibleRuntime(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/chat" {
			t.Fatalf("path = %q, muốn /api/chat", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"message":{"content":"Phản hồi theo nghiệp vụ riêng"}}`))
	}))
	defer server.Close()

	client := NewClient(nil)
	result, err := client.Complete(
		context.Background(),
		botsdomain.AIConfig{
			Provider: "ollama",
			Model:    "qwen2.5:7b",
			Settings: []byte(`{"base_url":"` + server.URL + `"}`),
		},
		botsdomain.Flow{Prompt: "Bạn là trợ lý nhân sự."},
		botauto.MessageInput{Body: "Tôi còn bao nhiêu ngày phép?"},
	)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if result != "Phản hồi theo nghiệp vụ riêng" {
		t.Fatalf("Complete() = %q", result)
	}
}

func TestResolveSecretRejectsUnscopedEnvironmentVariable(t *testing.T) {
	ref := "env://DATABASE_URL"
	_, err := resolveSecret(&ref)
	if err == nil || !strings.Contains(err.Error(), "BOT_AI_") {
		t.Fatalf("resolveSecret() error = %v, muốn lỗi giới hạn BOT_AI_", err)
	}
}
