package application

import (
	"encoding/json"
	"testing"
)

func TestValidateRunnerPayloadHTTP(t *testing.T) {
	service := NewService(nil, nil)
	payload := json.RawMessage(`{"method":"POST","url":"http://localhost:8080/health","headers":{"X-Test":"ok"},"body":{"ping":true}}`)

	if err := service.validateRunnerPayload("http", payload); err != nil {
		t.Fatalf("validateRunnerPayload trả lỗi: %v", err)
	}
}

func TestValidateRunnerPayloadCleanupAllowlist(t *testing.T) {
	service := NewService(nil, nil)
	payload := json.RawMessage(`{"task":"cleanup_expired_sessions","older_than":"24h"}`)

	if err := service.validateRunnerPayload("builtin_cleanup", payload); err != nil {
		t.Fatalf("validateRunnerPayload trả lỗi: %v", err)
	}
}

func TestValidateRunnerPayloadRejectsUnknownCleanup(t *testing.T) {
	service := NewService(nil, nil)
	payload := json.RawMessage(`{"task":"rm_all","older_than":"24h"}`)

	if err := service.validateRunnerPayload("builtin_cleanup", payload); err == nil {
		t.Fatal("validateRunnerPayload phải từ chối cleanup ngoài allowlist")
	}
}

func TestValidateRunnerPayloadScriptAllowlist(t *testing.T) {
	service := NewService(nil, nil, WithScriptAllowlist(map[string]string{"ticket": "C:\\tmp\\ticket.ps1"}))
	payload := json.RawMessage(`{"name":"ticket","args":["--severity","warning"],"timeout":"30s"}`)

	if err := service.validateRunnerPayload("script", payload); err != nil {
		t.Fatalf("validateRunnerPayload trả lỗi: %v", err)
	}
}

func TestValidateRunnerPayloadRejectsScriptOutsideAllowlist(t *testing.T) {
	service := NewService(nil, nil)
	payload := json.RawMessage(`{"name":"ticket","args":[]}`)

	if err := service.validateRunnerPayload("script", payload); err == nil {
		t.Fatal("validateRunnerPayload phải từ chối script ngoài allowlist")
	}
}
