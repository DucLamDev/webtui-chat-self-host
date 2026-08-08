package http

import (
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authapp "github.com/duclamdev/application-chat/backend/internal/modules/auth/application"
	"github.com/gin-gonic/gin"
)

func TestLegalDocumentsUsesConfiguredVersions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(nethttp.MethodGet, "/api/v1/auth/legal-documents", nil)
	service := authapp.NewService(nil, nil)
	service.SetLegalDocumentVersions("terms-v2", "privacy-v3")

	NewHandler(service).LegalDocuments(context)

	if recorder.Code != nethttp.StatusOK || !strings.Contains(recorder.Body.String(), `"version":"terms-v2"`) ||
		!strings.Contains(recorder.Body.String(), `"version":"privacy-v3"`) {
		t.Fatalf("legal documents response = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestSelfHostedAuthUsesConfiguredInstanceDomain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	request := httptest.NewRequest(nethttp.MethodPost, "https://chat.company.example/api/v1/auth/login", nil)
	request.Host = "forged-host.example"
	context.Request = request

	handler := NewHandler(nil)
	handler.SetInstanceDomain("chat.company.example")

	if got := handler.authRequestDomain(context, "other-company.example"); got != "chat.company.example" {
		t.Fatalf("auth request domain = %q, want configured instance domain", got)
	}
}

func TestSharedAuthCanUseExplicitDomain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(nethttp.MethodPost, "https://api.webtuichat.com/api/v1/auth/login", nil)

	handler := NewHandler(nil)

	if got := handler.authRequestDomain(context, "chat.company.example"); got != "chat.company.example" {
		t.Fatalf("auth request domain = %q, want explicit domain", got)
	}
}

func TestLegalAcceptanceWorkspaceScopePrefersExplicitAndFallsBackToToken(t *testing.T) {
	if got := legalAcceptanceWorkspaceID(" workspace-selected ", "workspace-token"); got != "workspace-selected" {
		t.Fatalf("explicit workspace = %q", got)
	}
	if got := legalAcceptanceWorkspaceID("", " workspace-token "); got != "workspace-token" {
		t.Fatalf("fallback workspace = %q", got)
	}
}
