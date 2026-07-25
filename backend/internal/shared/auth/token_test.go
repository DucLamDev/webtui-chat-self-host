package auth

import (
	"errors"
	"testing"
	"time"
)

func TestAccessTokenRoundTrip(t *testing.T) {
	manager := NewManager("access_secret_du_32_ky_tu_de_test", "refresh_secret_du_32_ky_tu_de_test", time.Minute, time.Hour)

	token, expiresAt, err := manager.CreateAccessToken("user-1", "a@example.com", "alice")
	if err != nil {
		t.Fatalf("CreateAccessToken() error = %v", err)
	}
	if expiresAt.IsZero() {
		t.Fatal("expiresAt không được rỗng")
	}

	claims, err := manager.VerifyAccessToken(token)
	if err != nil {
		t.Fatalf("VerifyAccessToken() error = %v", err)
	}
	if claims.Subject != "user-1" || claims.Email != "a@example.com" || claims.Username != "alice" {
		t.Fatalf("claims không đúng: %+v", claims)
	}
}

func TestAccessTokenRejectsTampering(t *testing.T) {
	manager := NewManager("access_secret_du_32_ky_tu_de_test", "refresh_secret_du_32_ky_tu_de_test", time.Minute, time.Hour)

	token, _, err := manager.CreateAccessToken("user-1", "a@example.com", "alice")
	if err != nil {
		t.Fatalf("CreateAccessToken() error = %v", err)
	}

	_, err = manager.VerifyAccessToken(token + "x")
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("VerifyAccessToken() error = %v, want ErrInvalidToken", err)
	}
}

func TestZoneAccessTokenRoundTrip(t *testing.T) {
	manager := NewManager("access_secret_du_32_ky_tu_de_test", "refresh_secret_du_32_ky_tu_de_test", time.Minute, time.Hour)

	token, _, err := manager.CreateZoneAccessToken(
		"user-1",
		"a@example.com",
		"alice",
		"zone-1",
		"workspace-1",
		"CHAT.Customer.Example",
	)
	if err != nil {
		t.Fatalf("CreateZoneAccessToken() error = %v", err)
	}
	claims, err := manager.VerifyAccessToken(token)
	if err != nil {
		t.Fatalf("VerifyAccessToken() error = %v", err)
	}
	if claims.ZoneID != "zone-1" || claims.WorkspaceID != "workspace-1" || claims.Domain != "chat.customer.example" {
		t.Fatalf("zone claims = %+v", claims)
	}
}
