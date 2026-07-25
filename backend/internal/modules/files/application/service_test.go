package application

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type permissionCheckerFunc func(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error)

func (fn permissionCheckerFunc) HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error) {
	return fn(ctx, userID, workspaceID, permissionCode)
}

func TestValidateUploadRejectsDangerousMimeType(t *testing.T) {
	_, _, _, err := validateUpload("tool.exe", "application/x-msdownload", 128, nil)
	if err == nil {
		t.Fatal("validateUpload() phải chặn MIME nguy hiểm")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok {
		t.Fatalf("lỗi = %T, muốn AppError", err)
	}
	if appErr.Code != "VALIDATION_ERROR" {
		t.Fatalf("mã lỗi = %q", appErr.Code)
	}
}

func TestValidateUploadAcceptsMetadataJSON(t *testing.T) {
	_, _, metadata, err := validateUpload("bao-cao.pdf", "application/pdf", 1024, json.RawMessage(`{"module":"test"}`))
	if err != nil {
		t.Fatalf("validateUpload() trả lỗi: %v", err)
	}
	if string(metadata) != `{"module":"test"}` {
		t.Fatalf("metadata = %s", metadata)
	}
}

func TestValidateUploadAcceptsBrowserVoiceFormats(t *testing.T) {
	formats := []struct {
		name     string
		mimeType string
	}{
		{name: "voice.webm", mimeType: "audio/webm;codecs=opus"},
		{name: "voice.ogg", mimeType: "audio/ogg;codecs=opus"},
		{name: "voice.m4a", mimeType: "audio/mp4"},
	}

	for _, format := range formats {
		t.Run(format.mimeType, func(t *testing.T) {
			_, mimeType, _, err := validateUpload(format.name, format.mimeType, 1024, json.RawMessage(`{"media_type":"voice"}`))
			if err != nil {
				t.Fatalf("validateUpload() trả lỗi: %v", err)
			}
			if strings.Contains(mimeType, ";") {
				t.Fatalf("mime type chưa được chuẩn hóa: %q", mimeType)
			}
		})
	}
}

func TestEnsureAnyPermissionAllowsMessageSendFallback(t *testing.T) {
	service := NewService(nil, nil, permissionCheckerFunc(func(_ context.Context, _ string, _ string, permissionCode string) (bool, error) {
		return permissionCode == "message.send", nil
	}))

	if err := service.ensureAnyPermission(context.Background(), "user-1", "workspace-1", "file.upload", "message.send"); err != nil {
		t.Fatalf("ensureAnyPermission() returned error: %v", err)
	}
}

func TestEnsureAnyPermissionRejectsWhenNoPermissionMatches(t *testing.T) {
	service := NewService(nil, nil, permissionCheckerFunc(func(_ context.Context, _ string, _ string, _ string) (bool, error) {
		return false, nil
	}))

	err := service.ensureAnyPermission(context.Background(), "user-1", "workspace-1", "file.upload", "message.send")
	if err == nil {
		t.Fatal("ensureAnyPermission() returned nil error")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok {
		t.Fatalf("error = %T, want AppError", err)
	}
	if appErr.Code != "FORBIDDEN" {
		t.Fatalf("error code = %q", appErr.Code)
	}
}

func TestNewObjectKeySanitizesFilename(t *testing.T) {
	key, err := newObjectKey("workspace-1", "../Báo cáo quý 1.pdf", "files", func() time.Time {
		return time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatalf("newObjectKey() trả lỗi: %v", err)
	}
	if strings.Contains(key, "..") || strings.Contains(key, "\\") {
		t.Fatalf("object key không an toàn: %s", key)
	}
	if !strings.HasPrefix(key, "workspaces/workspace-1/files/2026/07/") {
		t.Fatalf("object key = %s", key)
	}
}
