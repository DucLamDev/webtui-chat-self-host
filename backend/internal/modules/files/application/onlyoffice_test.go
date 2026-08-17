package application

import (
	"context"
	"strings"
	"testing"
	"time"

	filesdomain "github.com/duclamdev/application-chat/backend/internal/modules/files/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

func TestCreateOnlyOfficeSessionUsesAccessTokenQuery(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	service := NewService(onlyOfficeRepoStub{file: filesdomain.File{
		ID:           "file-1",
		OriginalName: "016 - PHIEU YEU CAU.doc",
		MimeType:     "application/msword",
		ByteSize:     191 << 10,
		UpdatedAt:    now,
	}}, nil, permissionCheckerFunc(func(_ context.Context, _ string, _ string, permissionCode string) (bool, error) {
		return permissionCode == "file.upload", nil
	}))
	service.now = func() time.Time { return now }
	service.SetOnlyOfficeOptions(OnlyOfficeOptions{
		Enabled:       true,
		PublicURL:     "https://chat.example.com:8444",
		APIBaseURL:    "http://api:8080",
		JWTSecret:     "onlyoffice-jwt-secret-for-tests",
		SessionSecret: "onlyoffice-session-secret-for-tests",
	})

	session, err := service.CreateOnlyOfficeSession(context.Background(), OnlyOfficeSessionInput{
		ActorUserID: "user-1",
		WorkspaceID: "workspace-1",
		FileID:      "file-1",
		UserName:    "Lam Ho",
	})
	if err != nil {
		t.Fatalf("CreateOnlyOfficeSession() returned error: %v", err)
	}

	document := session.Config["document"].(map[string]any)
	editorConfig := session.Config["editorConfig"].(map[string]any)
	for label, rawURL := range map[string]string{
		"document.url":             document["url"].(string),
		"editorConfig.callbackUrl": editorConfig["callbackUrl"].(string),
	} {
		if !strings.Contains(rawURL, "access_token=") {
			t.Fatalf("%s = %q, want access_token query", label, rawURL)
		}
		if strings.Contains(rawURL, "?token=") || strings.Contains(rawURL, "&token=") {
			t.Fatalf("%s = %q, must not use token query", label, rawURL)
		}
	}
}

func TestOnlyOfficeResultURLRewritesPublicURLToInternalURL(t *testing.T) {
	service := &Service{onlyOffice: OnlyOfficeOptions{
		PublicURL:   "https://chat.example.com:8444",
		InternalURL: "http://onlyoffice-document-server",
	}}

	got, err := service.onlyOfficeResultURL("https://chat.example.com:8444/cache/files/result.docx?token=abc")
	if err != nil {
		t.Fatalf("onlyOfficeResultURL() returned error: %v", err)
	}
	want := "http://onlyoffice-document-server/cache/files/result.docx?token=abc"
	if got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}
}

func TestOnlyOfficeResultURLRejectsUnexpectedHosts(t *testing.T) {
	service := &Service{onlyOffice: OnlyOfficeOptions{
		PublicURL:   "https://chat.example.com:8444",
		InternalURL: "http://onlyoffice-document-server",
	}}

	_, err := service.onlyOfficeResultURL("http://169.254.169.254/latest/meta-data")
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Code != "ONLYOFFICE_INVALID_RESULT_URL" {
		t.Fatalf("expected ONLYOFFICE_INVALID_RESULT_URL, got %#v", err)
	}
}

type onlyOfficeRepoStub struct {
	file filesdomain.File
}

func (r onlyOfficeRepoStub) CreateFile(context.Context, CreateFileParams) (filesdomain.File, error) {
	panic("not implemented")
}

func (r onlyOfficeRepoStub) FindFile(context.Context, string, string) (filesdomain.File, error) {
	return r.file, nil
}

func (r onlyOfficeRepoStub) CanAccessFile(context.Context, string, string, string) (bool, error) {
	return true, nil
}

func (r onlyOfficeRepoStub) ListFiles(context.Context, ListFilesParams) ([]filesdomain.File, error) {
	panic("not implemented")
}

func (r onlyOfficeRepoStub) CreateVersion(context.Context, CreateVersionParams) (filesdomain.Version, error) {
	panic("not implemented")
}

func (r onlyOfficeRepoStub) ListVersions(context.Context, string, string) ([]filesdomain.Version, error) {
	panic("not implemented")
}

func (r onlyOfficeRepoStub) AttachFile(context.Context, AttachFileParams) (filesdomain.Attachment, error) {
	panic("not implemented")
}

func (r onlyOfficeRepoStub) ListAttachments(context.Context, ListAttachmentsParams) ([]filesdomain.Attachment, error) {
	panic("not implemented")
}

func (r onlyOfficeRepoStub) ListChannelMedia(context.Context, ListChannelMediaParams) ([]filesdomain.Attachment, error) {
	panic("not implemented")
}

func (r onlyOfficeRepoStub) RecordAudit(context.Context, AuditEvent) error {
	return nil
}
