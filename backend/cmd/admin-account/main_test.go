package main

import (
	"testing"

	sharedauth "github.com/duclamdev/application-chat/backend/internal/shared/auth"
)

func TestGeneratePassword(t *testing.T) {
	password, err := generatePassword()
	if err != nil {
		t.Fatalf("generatePassword() error = %v", err)
	}
	if len(password) < 30 {
		t.Fatalf("generated password is too short: %d", len(password))
	}
	hash, err := sharedauth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if !sharedauth.VerifyPassword(hash, password) {
		t.Fatal("generated password cannot be verified")
	}
}

func TestParseAccountFlags(t *testing.T) {
	t.Setenv("INSTANCE_DOMAIN", "chat.example.com")
	options, err := parseAccountFlags("ensure", []string{"--username", "Admin_01", "--display-name", "Quản trị"})
	if err != nil {
		t.Fatalf("parseAccountFlags() error = %v", err)
	}
	if options.username != "admin_01" || options.email != "admin@chat.example.com" {
		t.Fatalf("unexpected options: %#v", options)
	}
}

func TestParseAccountFlagsRejectsUnsafeUsername(t *testing.T) {
	if _, err := parseAccountFlags("ensure", []string{"--username", "admin account"}); err == nil {
		t.Fatal("parseAccountFlags() accepted an unsafe username")
	}
}
