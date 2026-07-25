package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	tokenTypeAccess = "access"

	defaultAccessSecret  = "dev_access_secret_local_only_do_not_use_in_production"
	defaultRefreshSecret = "dev_refresh_secret_local_only_do_not_use_in_production"
)

var (
	ErrInvalidToken = errors.New("token không hợp lệ")
	ErrExpiredToken = errors.New("token đã hết hạn")
)

type Manager struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
	now           func() time.Time
}

type AccessClaims struct {
	Subject     string `json:"sub"`
	Email       string `json:"email,omitempty"`
	Username    string `json:"username,omitempty"`
	ZoneID      string `json:"zone_id,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Domain      string `json:"domain,omitempty"`
	Type        string `json:"typ"`
	IssuedAt    int64  `json:"iat"`
	ExpiresAt   int64  `json:"exp"`
}

func NewManager(accessSecret string, refreshSecret string, accessTTL time.Duration, refreshTTL time.Duration) *Manager {
	if strings.TrimSpace(accessSecret) == "" {
		accessSecret = defaultAccessSecret
	}
	if strings.TrimSpace(refreshSecret) == "" {
		refreshSecret = defaultRefreshSecret
	}
	if accessTTL <= 0 {
		accessTTL = 15 * time.Minute
	}
	if refreshTTL <= 0 {
		refreshTTL = 30 * 24 * time.Hour
	}

	return &Manager{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
		now:           time.Now,
	}
}

func (m *Manager) CreateAccessToken(userID string, email string, username string) (string, time.Time, error) {
	return m.CreateZoneAccessToken(userID, email, username, "", "", "")
}

func (m *Manager) CreateZoneAccessToken(
	userID string,
	email string,
	username string,
	zoneID string,
	workspaceID string,
	domain string,
) (string, time.Time, error) {
	now := m.now().UTC()
	expiresAt := now.Add(m.accessTTL)
	claims := AccessClaims{
		Subject:     userID,
		Email:       email,
		Username:    username,
		ZoneID:      strings.TrimSpace(zoneID),
		WorkspaceID: strings.TrimSpace(workspaceID),
		Domain:      strings.ToLower(strings.TrimSpace(domain)),
		Type:        tokenTypeAccess,
		IssuedAt:    now.Unix(),
		ExpiresAt:   expiresAt.Unix(),
	}

	token, err := m.sign(claims)
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

func (m *Manager) VerifyAccessToken(token string) (*AccessClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	message := parts[0] + "." + parts[1]
	expected := signHMAC([]byte(message), m.accessSecret)
	got, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(got, expected) {
		return nil, ErrInvalidToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims AccessClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	if claims.Type != tokenTypeAccess || claims.Subject == "" {
		return nil, ErrInvalidToken
	}
	if m.now().UTC().Unix() >= claims.ExpiresAt {
		return nil, ErrExpiredToken
	}

	return &claims, nil
}

func (m *Manager) NewRefreshToken() (string, time.Time, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", time.Time{}, err
	}
	token := "wtr_" + base64.RawURLEncoding.EncodeToString(b[:])
	return token, m.now().UTC().Add(m.refreshTTL), nil
}

func (m *Manager) HashRefreshToken(token string) string {
	mac := hmac.New(sha256.New, m.refreshSecret)
	_, _ = mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}

func (m *Manager) AccessTTL() time.Duration {
	return m.accessTTL
}

func (m *Manager) RefreshTTL() time.Duration {
	return m.refreshTTL
}

func (m *Manager) sign(claims AccessClaims) (string, error) {
	headerBytes, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", fmt.Errorf("mã hóa header token: %w", err)
	}
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("mã hóa payload token: %w", err)
	}

	header := base64.RawURLEncoding.EncodeToString(headerBytes)
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	message := header + "." + payload
	signature := base64.RawURLEncoding.EncodeToString(signHMAC([]byte(message), m.accessSecret))
	return message + "." + signature, nil
}

func signHMAC(message []byte, secret []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(message)
	return mac.Sum(nil)
}
