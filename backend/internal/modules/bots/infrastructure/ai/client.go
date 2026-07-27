package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	botsdomain "github.com/duclamdev/application-chat/backend/internal/modules/bots/domain"
	"github.com/duclamdev/application-chat/backend/internal/shared/botauto"
	"github.com/duclamdev/application-chat/backend/internal/shared/botsecrets"
)

const maxResponseBytes = 2 << 20

var botAIEnvironmentSecret = regexp.MustCompile(`^BOT_AI_[A-Z0-9_]+$`)

type Client struct {
	httpClient   *http.Client
	allowedHosts map[string]struct{}
	secretKey    string
}

type runtimeSettings struct {
	BaseURL     string  `json:"base_url"`
	Temperature float64 `json:"temperature"`
}

type chatResponse struct {
	Content string `json:"content"`
	Message any    `json:"message"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func NewClient(additionalAllowedHosts []string, secretKeys ...string) *Client {
	allowed := map[string]struct{}{
		"127.0.0.1":            {},
		"localhost":            {},
		"::1":                  {},
		"ollama":               {},
		"local-ai":             {},
		"localai":              {},
		"api.openai.com":       {},
		"host.docker.internal": {},
	}
	for _, host := range additionalAllowedHosts {
		host = strings.ToLower(strings.TrimSpace(host))
		if host != "" {
			allowed[host] = struct{}{}
		}
	}
	client := &Client{
		httpClient:   &http.Client{Timeout: 90 * time.Second},
		allowedHosts: allowed,
	}
	if len(secretKeys) > 0 {
		client.secretKey = strings.TrimSpace(secretKeys[0])
	}
	return client
}

func (c *Client) Complete(
	ctx context.Context,
	config botsdomain.AIConfig,
	flow botsdomain.Flow,
	input botauto.MessageInput,
) (string, error) {
	var settings runtimeSettings
	if len(config.Settings) > 0 {
		if err := json.Unmarshal(config.Settings, &settings); err != nil {
			return "", errors.New("cấu hình AI không phải JSON hợp lệ")
		}
	}
	temperature := settings.Temperature
	if temperature < 0 || temperature > 2 {
		temperature = 0.2
	}
	systemPrompt := buildSystemPrompt(flow)
	provider := strings.ToLower(strings.TrimSpace(config.Provider))
	model := strings.TrimSpace(config.Model)
	if model == "" {
		return "", errors.New("bot chưa có model AI")
	}
	secret, err := resolveSecretWithMaster(config.SecretRef, c.secretKey)
	if err != nil {
		return "", err
	}

	switch provider {
	case "ollama":
		endpoint, endpointErr := c.safeEndpoint(firstNonEmpty(settings.BaseURL, "http://ollama:11434"), "/api/chat")
		if endpointErr != nil {
			return "", endpointErr
		}
		payload := map[string]any{
			"messages": []map[string]string{
				{"role": "system", "content": systemPrompt},
				{"role": "user", "content": input.Body},
			},
			"model":   model,
			"options": map[string]any{"temperature": temperature},
			"stream":  false,
		}
		var response struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}
		if err := c.postJSON(ctx, endpoint, secret, payload, &response); err != nil {
			return "", err
		}
		return requireContent(response.Message.Content)
	case "localai", "local_ai", "openai_compatible":
		endpoint, endpointErr := c.safeEndpoint(firstNonEmpty(settings.BaseURL, "http://local-ai:8080"), "/v1/chat/completions")
		if endpointErr != nil {
			return "", endpointErr
		}
		payload := map[string]any{
			"messages": []map[string]string{
				{"role": "system", "content": systemPrompt},
				{"role": "user", "content": input.Body},
			},
			"model":       model,
			"temperature": temperature,
		}
		var response chatResponse
		if err := c.postJSON(ctx, endpoint, secret, payload, &response); err != nil {
			return "", err
		}
		if len(response.Choices) == 0 {
			return "", errors.New("nhà cung cấp AI không trả về lựa chọn nào")
		}
		return requireContent(response.Choices[0].Message.Content)
	case "webhook":
		endpoint, endpointErr := c.safeEndpoint(settings.BaseURL, "")
		if endpointErr != nil {
			return "", endpointErr
		}
		payload := map[string]any{
			"bot_id":           config.BotID,
			"flow_id":          flow.ID,
			"knowledge_config": json.RawMessage(defaultJSON(flow.KnowledgeConfig)),
			"message":          input.Body,
			"model":            model,
			"prompt":           flow.Prompt,
			"tool_config":      json.RawMessage(defaultJSON(flow.ToolConfig)),
			"trigger_config":   json.RawMessage(defaultJSON(flow.TriggerConfig)),
			"workspace_id":     input.WorkspaceID,
		}
		var response chatResponse
		if err := c.postJSON(ctx, endpoint, secret, payload, &response); err != nil {
			return "", err
		}
		if strings.TrimSpace(response.Content) != "" {
			return strings.TrimSpace(response.Content), nil
		}
		if message, ok := response.Message.(string); ok {
			return requireContent(message)
		}
		if len(response.Choices) > 0 {
			return requireContent(response.Choices[0].Message.Content)
		}
		return "", errors.New("webhook bot không trả về trường content hoặc message")
	default:
		return "", fmt.Errorf("provider AI %q chưa được hỗ trợ", config.Provider)
	}
}

func buildSystemPrompt(flow botsdomain.Flow) string {
	parts := []string{strings.TrimSpace(flow.Prompt)}
	if config := strings.TrimSpace(string(flow.KnowledgeConfig)); config != "" && config != "{}" {
		parts = append(parts, "Nguồn kiến thức được cấu hình (JSON):\n"+config)
	}
	if config := strings.TrimSpace(string(flow.ToolConfig)); config != "" && config != "{}" {
		parts = append(parts, "Công cụ được phép sử dụng (JSON):\n"+config)
	}
	parts = append(parts, "Chỉ trả lời theo nghiệp vụ và dữ liệu được cấp. Không tự bịa thông tin.")
	return strings.Join(parts, "\n\n")
}

func (c *Client) postJSON(ctx context.Context, endpoint string, secret string, payload any, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if secret != "" {
		request.Header.Set("Authorization", "Bearer "+secret)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, maxResponseBytes)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		detail, _ := io.ReadAll(limited)
		return fmt.Errorf("nhà cung cấp AI trả về HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(detail)))
	}
	if err := json.NewDecoder(limited).Decode(target); err != nil {
		return fmt.Errorf("không đọc được phản hồi AI: %w", err)
	}
	return nil
}

func (c *Client) safeEndpoint(baseURL string, path string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil || parsed.Hostname() == "" || parsed.User != nil {
		return "", errors.New("endpoint AI không hợp lệ")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("endpoint AI phải dùng HTTP hoặc HTTPS")
	}
	host := strings.ToLower(parsed.Hostname())
	_, explicitlyAllowed := c.allowedHosts[host]
	ip := net.ParseIP(host)
	if !explicitlyAllowed && (ip == nil || !(ip.IsPrivate() || ip.IsLoopback())) {
		return "", fmt.Errorf("host AI %q chưa nằm trong BOT_AI_ALLOWED_HOSTS", host)
	}
	if path != "" {
		parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func resolveSecret(secretRef *string) (string, error) {
	if secretRef == nil || strings.TrimSpace(*secretRef) == "" {
		return "", nil
	}
	ref := strings.TrimSpace(*secretRef)
	if !strings.HasPrefix(ref, "env://") {
		return "", errors.New("runtime hiện chỉ hỗ trợ secret_ref dạng env://BOT_AI_*")
	}
	name := strings.TrimPrefix(ref, "env://")
	if !botAIEnvironmentSecret.MatchString(name) {
		return "", errors.New("secret_ref phải trỏ đến biến môi trường bắt đầu bằng BOT_AI_")
	}
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return "", fmt.Errorf("biến môi trường %s chưa được cấu hình", name)
	}
	return value, nil
}

func resolveSecretWithMaster(secretRef *string, masterSecret string) (string, error) {
	if secretRef != nil && botsecrets.IsEncrypted(*secretRef) {
		value, err := botsecrets.Decrypt(masterSecret, *secretRef)
		if err != nil {
			return "", errors.New("không giải mã được API key đã lưu của bot")
		}
		return value, nil
	}
	return resolveSecret(secretRef)
}

func defaultJSON(value []byte) []byte {
	if len(value) == 0 || !json.Valid(value) {
		return []byte(`{}`)
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func requireContent(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("nhà cung cấp AI trả về nội dung trống")
	}
	return value, nil
}
