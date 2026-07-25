package fcm

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
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

const firebaseMessagingScope = "https://www.googleapis.com/auth/firebase.messaging"

type Config struct {
	ProjectID                string
	ServiceAccountFile       string
	ServiceAccountJSONBase64 string
	HTTPClient               *http.Client
}

type Sender struct {
	projectID   string
	client      *http.Client
	credential  serviceAccount
	mu          sync.Mutex
	accessToken string
	expiresAt   time.Time
	initErr     error
}

type serviceAccount struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	ProjectID   string `json:"project_id"`
	TokenURI    string `json:"token_uri"`
}

type oauthToken struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

func NewSender(config Config) *Sender {
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	sender := &Sender{client: client, projectID: strings.TrimSpace(config.ProjectID)}
	raw, err := readCredentials(config)
	if err != nil {
		sender.initErr = err
		return sender
	}
	if len(raw) == 0 {
		return sender
	}
	if err := json.Unmarshal(raw, &sender.credential); err != nil {
		sender.initErr = fmt.Errorf("decode Firebase service account: %w", err)
		return sender
	}
	if sender.projectID == "" {
		sender.projectID = strings.TrimSpace(sender.credential.ProjectID)
	}
	if sender.credential.TokenURI == "" {
		sender.credential.TokenURI = "https://oauth2.googleapis.com/token"
	}
	if sender.projectID == "" || strings.TrimSpace(sender.credential.ClientEmail) == "" || strings.TrimSpace(sender.credential.PrivateKey) == "" {
		sender.initErr = errors.New("Firebase service account is missing project_id, client_email, or private_key")
	}
	return sender
}

func (s *Sender) Enabled() bool {
	return s != nil && s.initErr == nil && s.projectID != "" && s.credential.ClientEmail != ""
}

func (s *Sender) Send(ctx context.Context, token string, payload map[string]any) error {
	if !s.Enabled() {
		if s != nil && s.initErr != nil {
			return s.initErr
		}
		return errors.New("Firebase Cloud Messaging is not configured")
	}
	deviceToken := strings.TrimSpace(token)
	if deviceToken == "" {
		return errors.New("FCM device token is empty")
	}
	accessToken, err := s.token(ctx)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{"message": buildMessage(deviceToken, payload)})
	if err != nil {
		return err
	}
	endpoint := "https://fcm.googleapis.com/v1/projects/" + url.PathEscape(s.projectID) + "/messages:send"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		_, _ = io.Copy(io.Discard, response.Body)
		return nil
	}
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return fmt.Errorf("FCM returned %s: %s", response.Status, strings.TrimSpace(string(raw)))
}

func (s *Sender) token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.accessToken != "" && time.Now().Add(time.Minute).Before(s.expiresAt) {
		return s.accessToken, nil
	}
	assertion, err := s.jwtAssertion()
	if err != nil {
		return "", err
	}
	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {assertion},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.credential.TokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return "", fmt.Errorf("Firebase OAuth returned %s: %s", response.Status, strings.TrimSpace(string(raw)))
	}
	var result oauthToken
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", err
	}
	if strings.TrimSpace(result.AccessToken) == "" {
		return "", errors.New("Firebase OAuth returned an empty access token")
	}
	s.accessToken = result.AccessToken
	expiresIn := result.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	s.expiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
	return s.accessToken, nil
}

func (s *Sender) jwtAssertion() (string, error) {
	block, _ := pem.Decode([]byte(s.credential.PrivateKey))
	if block == nil {
		return "", errors.New("Firebase private key is not valid PEM")
	}
	privateKey, err := parsePrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}
	now := time.Now().Unix()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	claims, _ := json.Marshal(map[string]any{
		"iss":   s.credential.ClientEmail,
		"scope": firebaseMessagingScope,
		"aud":   s.credential.TokenURI,
		"iat":   now,
		"exp":   now + 3600,
	})
	unsigned := encodeSegment(header) + "." + encodeSegment(claims)
	digest := crypto.SHA256.New()
	_, _ = digest.Write([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest.Sum(nil))
	if err != nil {
		return "", err
	}
	return unsigned + "." + encodeSegment(signature), nil
}

func parsePrivateKey(raw []byte) (*rsa.PrivateKey, error) {
	if parsed, err := x509.ParsePKCS8PrivateKey(raw); err == nil {
		if key, ok := parsed.(*rsa.PrivateKey); ok {
			return key, nil
		}
	}
	key, err := x509.ParsePKCS1PrivateKey(raw)
	if err != nil {
		return nil, fmt.Errorf("parse Firebase private key: %w", err)
	}
	return key, nil
}

func buildMessage(token string, payload map[string]any) map[string]any {
	data := make(map[string]string, len(payload))
	for key, value := range payload {
		if value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			data[key] = typed
		default:
			encoded, err := json.Marshal(typed)
			if err == nil {
				data[key] = string(encoded)
			}
		}
	}
	title := strings.TrimSpace(data["title"])
	body := strings.TrimSpace(data["body"])
	tag := strings.TrimSpace(data["tag"])
	eventType := strings.TrimSpace(data["event_type"])
	isCallEvent := strings.HasPrefix(eventType, "call_")
	category := "WEBTUI_MESSAGE"
	if isCallEvent {
		category = "WEBTUI_CALL"
	}
	androidNotification := map[string]any{
		"default_sound":           true,
		"default_vibrate_timings": true,
		"notification_priority":   "PRIORITY_MAX",
		"visibility":              "PUBLIC",
	}
	if tag != "" {
		androidNotification["tag"] = tag
	}
	androidConfig := map[string]any{
		"priority": "high",
		"ttl":      "35s",
	}
	if !isCallEvent {
		androidConfig["notification"] = androidNotification
	}
	if tag != "" {
		androidConfig["collapse_key"] = tag
	}
	message := map[string]any{
		"token":   token,
		"data":    data,
		"android": androidConfig,
		"apns": map[string]any{
			"headers": map[string]string{"apns-priority": "10"},
			"payload": map[string]any{
				"aps": map[string]any{
					"sound":             "default",
					"category":          category,
					"content-available": 1,
				},
			},
		},
	}
	if !isCallEvent && (title != "" || body != "") {
		message["notification"] = map[string]string{"title": title, "body": body}
	}
	return message
}

func readCredentials(config Config) ([]byte, error) {
	if encoded := strings.TrimSpace(config.ServiceAccountJSONBase64); encoded != "" {
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode FIREBASE_SERVICE_ACCOUNT_JSON_BASE64: %w", err)
		}
		return raw, nil
	}
	if path := strings.TrimSpace(config.ServiceAccountFile); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read Firebase service account file: %w", err)
		}
		return raw, nil
	}
	return nil, nil
}

func encodeSegment(raw []byte) string {
	return base64.RawURLEncoding.EncodeToString(raw)
}
