package application

import (
	"testing"

	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

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
