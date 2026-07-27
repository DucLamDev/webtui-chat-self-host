package localai

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
	"strings"
	"time"

	channelsapp "github.com/duclamdev/application-chat/backend/internal/modules/channels/application"
)

const maxAIResponseBytes = 2 << 20

type Client struct {
	httpClient   *http.Client
	allowedHosts map[string]struct{}
}

type integrationConfig struct {
	OllamaURL   string `json:"ollama_url"`
	OllamaModel string `json:"ollama_model"`
	LocalAIURL  string `json:"local_ai_url"`
	LocalModel  string `json:"local_ai_model"`
}

type structuredSummary struct {
	Summary     string   `json:"summary"`
	Decisions   []string `json:"decisions"`
	ActionItems []string `json:"action_items"`
}

func NewClient(additionalAllowedHosts []string) *Client {
	allowed := map[string]struct{}{
		"127.0.0.1":            {},
		"localhost":            {},
		"::1":                  {},
		"ollama":               {},
		"local-ai":             {},
		"localai":              {},
		"host.docker.internal": {},
	}
	for _, host := range additionalAllowedHosts {
		host = strings.ToLower(strings.TrimSpace(host))
		if host != "" {
			allowed[host] = struct{}{}
		}
	}
	return &Client{
		httpClient:   &http.Client{Timeout: 90 * time.Second},
		allowedHosts: allowed,
	}
}

func (c *Client) Summarize(
	ctx context.Context,
	provider string,
	rawConfig json.RawMessage,
	messages []channelsapp.TalkAISummaryMessage,
	language string,
) (channelsapp.TalkSummaryResult, error) {
	var config integrationConfig
	if len(rawConfig) > 0 {
		_ = json.Unmarshal(rawConfig, &config)
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = "ollama"
	}
	prompt, err := buildPrompt(messages, language)
	if err != nil {
		return channelsapp.TalkSummaryResult{}, err
	}
	switch provider {
	case "ollama":
		endpoint := firstNonEmpty(config.OllamaURL, "http://ollama:11434")
		model := firstNonEmpty(config.OllamaModel, "qwen2.5:7b-instruct")
		content, requestErr := c.callOllama(ctx, endpoint, model, prompt)
		if requestErr != nil {
			return channelsapp.TalkSummaryResult{}, requestErr
		}
		return parseSummary(content, model)
	case "local_ai", "localai":
		endpoint := firstNonEmpty(config.LocalAIURL, "http://local-ai:8080")
		model := firstNonEmpty(config.LocalModel, "qwen2.5-7b-instruct")
		content, requestErr := c.callOpenAICompatible(
			ctx,
			endpoint,
			model,
			prompt,
		)
		if requestErr != nil {
			return channelsapp.TalkSummaryResult{}, requestErr
		}
		return parseSummary(content, model)
	default:
		return channelsapp.TalkSummaryResult{}, fmt.Errorf(
			"unsupported local AI provider %q",
			provider,
		)
	}
}

func (c *Client) callOllama(
	ctx context.Context,
	baseURL string,
	model string,
	prompt string,
) (string, error) {
	endpoint, err := c.safeEndpoint(baseURL, "/api/chat")
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"model":  model,
		"stream": false,
		"format": "json",
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You summarize workplace conversations. Return only valid JSON.",
			},
			{"role": "user", "content": prompt},
		},
		"options": map[string]any{"temperature": 0.2},
	}
	var response struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := c.postJSON(ctx, endpoint, payload, &response); err != nil {
		return "", err
	}
	return response.Message.Content, nil
}

func (c *Client) callOpenAICompatible(
	ctx context.Context,
	baseURL string,
	model string,
	prompt string,
) (string, error) {
	endpoint, err := c.safeEndpoint(baseURL, "/v1/chat/completions")
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"model":       model,
		"temperature": 0.2,
		"response_format": map[string]string{
			"type": "json_object",
		},
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You summarize workplace conversations. Return only valid JSON.",
			},
			{"role": "user", "content": prompt},
		},
	}
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := c.postJSON(ctx, endpoint, payload, &response); err != nil {
		return "", err
	}
	if len(response.Choices) == 0 {
		return "", errors.New("local AI returned no choices")
	}
	return response.Choices[0].Message.Content, nil
}

func (c *Client) postJSON(
	ctx context.Context,
	endpoint string,
	payload any,
	target any,
) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, maxAIResponseBytes)
	if response.StatusCode < http.StatusOK ||
		response.StatusCode >= http.StatusMultipleChoices {
		detail, _ := io.ReadAll(limited)
		return fmt.Errorf(
			"local AI returned %d: %s",
			response.StatusCode,
			strings.TrimSpace(string(detail)),
		)
	}
	return json.NewDecoder(limited).Decode(target)
}

func (c *Client) safeEndpoint(baseURL string, path string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil || parsed.Hostname() == "" || parsed.User != nil {
		return "", errors.New("invalid local AI endpoint")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("local AI endpoint must use HTTP or HTTPS")
	}
	host := strings.ToLower(parsed.Hostname())
	_, explicitlyAllowed := c.allowedHosts[host]
	ip := net.ParseIP(host)
	if !explicitlyAllowed &&
		(ip == nil || !(ip.IsPrivate() || ip.IsLoopback())) {
		return "", errors.New("local AI endpoint host is not allowlisted")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func buildPrompt(
	messages []channelsapp.TalkAISummaryMessage,
	language string,
) (string, error) {
	encoded, err := json.Marshal(messages)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"Summarize the following conversation in language %q. "+
			"Return a JSON object with exactly these fields: "+
			"summary (string), decisions (array of strings), "+
			"action_items (array of strings). Do not invent facts.\n\n%s",
		language,
		encoded,
	), nil
}

func parseSummary(
	content string,
	model string,
) (channelsapp.TalkSummaryResult, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	var parsed structuredSummary
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &parsed); err != nil {
		return channelsapp.TalkSummaryResult{}, err
	}
	parsed.Summary = strings.TrimSpace(parsed.Summary)
	if parsed.Summary == "" {
		return channelsapp.TalkSummaryResult{}, errors.New("local AI returned an empty summary")
	}
	return channelsapp.TalkSummaryResult{
		Summary:     parsed.Summary,
		Decisions:   compactStrings(parsed.Decisions, 50),
		ActionItems: compactStrings(parsed.ActionItems, 50),
		Model:       model,
	}, nil
}

func compactStrings(values []string, limit int) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
		if len(result) >= limit {
			break
		}
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
