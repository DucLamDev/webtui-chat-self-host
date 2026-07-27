package apns

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type Config struct {
	KeyID            string
	TeamID           string
	BundleID         string
	PrivateKeyFile   string
	PrivateKeyBase64 string
	Sandbox          bool
}

type Sender struct {
	keyID    string
	teamID   string
	bundleID string
	host     string
	key      *ecdsa.PrivateKey
	client   *http.Client

	mu        sync.Mutex
	token     string
	expiresAt time.Time
	initErr   error
}

func NewSender(config Config) *Sender {
	host := "https://api.push.apple.com"
	if config.Sandbox {
		host = "https://api.sandbox.push.apple.com"
	}
	sender := &Sender{
		keyID:    strings.TrimSpace(config.KeyID),
		teamID:   strings.TrimSpace(config.TeamID),
		bundleID: strings.TrimSpace(config.BundleID),
		host:     host,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
	raw, err := readPrivateKey(config)
	if err != nil {
		sender.initErr = err
		return sender
	}
	sender.key, err = parsePrivateKey(raw)
	if err != nil {
		sender.initErr = err
	}
	if sender.keyID == "" || sender.teamID == "" || sender.bundleID == "" {
		sender.initErr = errors.New("APNs requires key_id, team_id, and bundle_id")
	}
	return sender
}

func (s *Sender) Provider() string {
	return "apns"
}

func (s *Sender) Enabled() bool {
	return s != nil && s.initErr == nil && s.key != nil
}

func (s *Sender) Send(ctx context.Context, deviceToken string, payload map[string]any) error {
	if eventType(payload) != "call_invite" {
		// VoIP tokens must only be used to initiate calls.
		return nil
	}
	if !s.Enabled() {
		if s != nil && s.initErr != nil {
			return s.initErr
		}
		return errors.New("APNs VoIP sender is not configured")
	}
	token := strings.TrimSpace(deviceToken)
	if token == "" {
		return errors.New("APNs device token is empty")
	}
	providerToken, err := s.providerToken()
	if err != nil {
		return err
	}
	body, err := json.Marshal(voipPayload(payload))
	if err != nil {
		return err
	}
	endpoint := s.host + "/3/device/" + url.PathEscape(token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("authorization", "bearer "+providerToken)
	req.Header.Set("apns-topic", s.bundleID+".voip")
	req.Header.Set("apns-push-type", "voip")
	req.Header.Set("apns-priority", "10")
	req.Header.Set("apns-expiration", "0")
	req.Header.Set("content-type", "application/json")
	response, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusOK {
		_, _ = io.Copy(io.Discard, response.Body)
		return nil
	}
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return fmt.Errorf("APNs returned %s: %s", response.Status, strings.TrimSpace(string(raw)))
}

func (s *Sender) providerToken() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if s.token != "" && now.Add(time.Minute).Before(s.expiresAt) {
		return s.token, nil
	}
	header, err := encodeJSON(map[string]string{"alg": "ES256", "kid": s.keyID})
	if err != nil {
		return "", err
	}
	claims, err := encodeJSON(map[string]any{"iss": s.teamID, "iat": now.Unix()})
	if err != nil {
		return "", err
	}
	signingInput := header + "." + claims
	digest := sha256.Sum256([]byte(signingInput))
	r, signatureS, err := ecdsa.Sign(rand.Reader, s.key, digest[:])
	if err != nil {
		return "", err
	}
	size := (elliptic.P256().Params().BitSize + 7) / 8
	signature := make([]byte, size*2)
	r.FillBytes(signature[:size])
	signatureS.FillBytes(signature[size:])
	s.token = signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
	s.expiresAt = now.Add(50 * time.Minute)
	return s.token, nil
}

func voipPayload(payload map[string]any) map[string]any {
	result := make(map[string]any, len(payload)+4)
	for key, value := range payload {
		result[key] = value
	}
	result["aps"] = map[string]any{}
	result["id"] = stringValue(payload["call_id"])
	result["nameCaller"] = firstString(payload["body"], payload["title"], "Cuộc gọi đến")
	result["handle"] = firstString(payload["title"], "Ứng dụng chat")
	result["appName"] = firstString(payload["app_name"], payload["appName"], "Ứng dụng chat")
	if strings.EqualFold(stringValue(payload["mode"]), "video") {
		result["type"] = 1
	} else {
		result["type"] = 0
	}
	result["extra"] = payload
	return result
}

func eventType(payload map[string]any) string {
	return strings.ToLower(strings.TrimSpace(stringValue(payload["event_type"])))
}

func firstString(values ...any) string {
	for _, value := range values {
		if text := strings.TrimSpace(stringValue(value)); text != "" {
			return text
		}
	}
	return ""
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func encodeJSON(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func readPrivateKey(config Config) ([]byte, error) {
	if encoded := strings.TrimSpace(config.PrivateKeyBase64); encoded != "" {
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode APNS_PRIVATE_KEY_BASE64: %w", err)
		}
		return raw, nil
	}
	if path := strings.TrimSpace(config.PrivateKeyFile); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read APNs private key: %w", err)
		}
		return raw, nil
	}
	return nil, errors.New("APNs private key is not configured")
}

func parsePrivateKey(raw []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("APNs private key is not valid PEM")
	}
	value, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse APNs private key: %w", err)
	}
	key, ok := value.(*ecdsa.PrivateKey)
	if !ok || key.Curve != elliptic.P256() {
		return nil, errors.New("APNs private key must be an ES256 P-256 key")
	}
	return key, nil
}
