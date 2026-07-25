package http

import (
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

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
