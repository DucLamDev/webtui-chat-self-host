package httpclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	orderapp "github.com/duclamdev/application-chat/backend/internal/modules/order/application"
)

func TestParseUpstreamErrorReadsJSONMessage(t *testing.T) {
	err := parseUpstreamError(http.StatusForbidden, []byte(`{
		"ok": false,
		"message": "IP không nằm trong whitelist: 160.191.55.144"
	}`))

	var upstream *orderapp.UpstreamError
	if !errors.As(err, &upstream) {
		t.Fatalf("error type = %T, want *application.UpstreamError", err)
	}
	if upstream.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", upstream.StatusCode, http.StatusForbidden)
	}
	if upstream.Message != "IP không nằm trong whitelist: 160.191.55.144" {
		t.Fatalf("message = %q", upstream.Message)
	}
}

func TestRenewServiceUsesInternalEndpointAndIdempotencyPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/internal/services/renew" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if request.Method != http.MethodPost || request.Header.Get("X-API-Key") != "internal-key" {
			t.Fatalf("method/header = %s/%q", request.Method, request.Header.Get("X-API-Key"))
		}
		var input orderapp.RenewServiceRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		if input.ServiceID != 1234 || input.IdempotencyKey != "message-123" || input.Months != 2 {
			t.Fatalf("input = %#v", input)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ok":true,"status":"success","data":{"outcome":"renewed","service_id":1234}}`))
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL, InternalAPIKey: "internal-key"})
	result, err := client.RenewService(context.Background(), orderapp.RenewServiceRequest{
		Email:          "khach@example.com",
		ServiceID:      1234,
		Months:         2,
		IdempotencyKey: "message-123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Data.Outcome != "renewed" || result.Data.ServiceID != 1234 {
		t.Fatalf("result = %#v", result)
	}
}
