package application

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	cronjobsdomain "github.com/duclamdev/application-chat/backend/internal/modules/cronjobs/domain"
)

const maxRunLogLength = 8000

type runResult struct {
	Status string
	Log    string
	Error  string
}

type httpPayload struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
}

type cleanupPayload struct {
	Task      string `json:"task"`
	OlderThan string `json:"older_than"`
}

type scriptPayload struct {
	Name    string   `json:"name"`
	Args    []string `json:"args"`
	Timeout string   `json:"timeout"`
}

func (s *Service) execute(ctx context.Context, job cronjobsdomain.CronJob) runResult {
	switch strings.TrimSpace(job.Runner) {
	case "http":
		return s.executeHTTP(ctx, job.Payload)
	case "builtin_cleanup", "worker":
		return s.executeCleanup(ctx, job.Payload)
	case "script":
		return s.executeScript(ctx, job.Payload)
	default:
		return runResult{
			Status: "failed",
			Error:  "Runner cronjob không được hỗ trợ.",
		}
	}
}

func (s *Service) executeHTTP(ctx context.Context, payload []byte) runResult {
	var reqPayload httpPayload
	if err := json.Unmarshal(payload, &reqPayload); err != nil {
		return runResult{Status: "failed", Error: "Payload HTTP không phải JSON hợp lệ."}
	}
	method := strings.ToUpper(strings.TrimSpace(reqPayload.Method))
	if method == "" {
		method = http.MethodPost
	}
	body := bytes.NewReader(reqPayload.Body)
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimSpace(reqPayload.URL), body)
	if err != nil {
		return runResult{Status: "failed", Error: err.Error()}
	}
	if len(reqPayload.Body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range reqPayload.Headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return runResult{Status: "failed", Error: err.Error()}
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxRunLogLength))
	logText := fmt.Sprintf("HTTP %s %s trả về status %d. Body: %s", method, req.URL.String(), resp.StatusCode, strings.TrimSpace(string(responseBody)))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return runResult{Status: "failed", Log: truncateRunLog(logText), Error: fmt.Sprintf("HTTP status %d", resp.StatusCode)}
	}
	return runResult{Status: "success", Log: truncateRunLog(logText)}
}

func (s *Service) executeCleanup(ctx context.Context, payload []byte) runResult {
	var reqPayload cleanupPayload
	if err := json.Unmarshal(payload, &reqPayload); err != nil {
		return runResult{Status: "failed", Error: "Payload cleanup không phải JSON hợp lệ."}
	}
	task := strings.TrimSpace(reqPayload.Task)
	olderThan := cleanupRetention(task)
	if strings.TrimSpace(reqPayload.OlderThan) != "" {
		parsed, err := time.ParseDuration(strings.TrimSpace(reqPayload.OlderThan))
		if err != nil || parsed < time.Hour {
			return runResult{Status: "failed", Error: "older_than phải là duration hợp lệ và tối thiểu 1h."}
		}
		olderThan = parsed
	}

	count, err := s.runCleanup(ctx, task, olderThan)
	if err != nil {
		return runResult{Status: "failed", Error: err.Error()}
	}
	return runResult{
		Status: "success",
		Log:    fmt.Sprintf("Cleanup %s hoàn tất, đã xử lý %d bản ghi.", task, count),
	}
}

func (s *Service) executeScript(ctx context.Context, payload []byte) runResult {
	var reqPayload scriptPayload
	if err := json.Unmarshal(payload, &reqPayload); err != nil {
		return runResult{Status: "failed", Error: "Payload script không phải JSON hợp lệ."}
	}
	name := strings.ToLower(strings.TrimSpace(reqPayload.Name))
	scriptPath := s.scriptAllowlist[name]
	if scriptPath == "" {
		return runResult{Status: "failed", Error: "Script không nằm trong allowlist."}
	}
	timeout := time.Minute
	if strings.TrimSpace(reqPayload.Timeout) != "" {
		parsed, err := time.ParseDuration(strings.TrimSpace(reqPayload.Timeout))
		if err != nil || parsed <= 0 || parsed > 10*time.Minute {
			return runResult{Status: "failed", Error: "timeout script phải hợp lệ và không quá 10m."}
		}
		timeout = parsed
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	command := exec.CommandContext(runCtx, scriptPath, reqPayload.Args...)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		if runCtx.Err() != nil {
			return runResult{Status: "failed", Log: truncateRunLog(output.String()), Error: "Script chạy quá thời gian cho phép."}
		}
		return runResult{Status: "failed", Log: truncateRunLog(output.String()), Error: err.Error()}
	}
	return runResult{Status: "success", Log: truncateRunLog(output.String())}
}

func (s *Service) runCleanup(ctx context.Context, task string, olderThan time.Duration) (int, error) {
	switch task {
	case "cleanup_expired_sessions":
		return s.repo.CleanupExpiredSessions(ctx, olderThan)
	case "cleanup_dead_outbox":
		return s.repo.CleanupDeadOutbox(ctx, olderThan)
	case "cleanup_failed_uploads":
		return s.repo.CleanupFailedUploads(ctx, olderThan)
	case "cleanup_old_presence":
		return s.repo.CleanupOldPresence(ctx, olderThan)
	default:
		return 0, fmt.Errorf("tác vụ cleanup không nằm trong allowlist")
	}
}

func (s *Service) validateRunnerPayload(runner string, payload []byte) error {
	switch runner {
	case "http":
		var reqPayload httpPayload
		if err := json.Unmarshal(payload, &reqPayload); err != nil {
			return fmt.Errorf("payload HTTP không phải JSON hợp lệ")
		}
		method := strings.ToUpper(strings.TrimSpace(reqPayload.Method))
		if method == "" {
			method = http.MethodPost
		}
		if !allowedHTTPMethod(method) {
			return fmt.Errorf("method HTTP không được hỗ trợ")
		}
		parsed, err := url.ParseRequestURI(strings.TrimSpace(reqPayload.URL))
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("url HTTP không hợp lệ")
		}
	case "builtin_cleanup", "worker":
		var reqPayload cleanupPayload
		if err := json.Unmarshal(payload, &reqPayload); err != nil {
			return fmt.Errorf("payload cleanup không phải JSON hợp lệ")
		}
		switch strings.TrimSpace(reqPayload.Task) {
		case "cleanup_expired_sessions", "cleanup_dead_outbox", "cleanup_failed_uploads", "cleanup_old_presence":
		default:
			return fmt.Errorf("tác vụ cleanup không nằm trong allowlist")
		}
		if strings.TrimSpace(reqPayload.OlderThan) != "" {
			parsed, err := time.ParseDuration(strings.TrimSpace(reqPayload.OlderThan))
			if err != nil || parsed < time.Hour {
				return fmt.Errorf("older_than phải là duration hợp lệ và tối thiểu 1h")
			}
		}
	case "script":
		var reqPayload scriptPayload
		if err := json.Unmarshal(payload, &reqPayload); err != nil {
			return fmt.Errorf("payload script không phải JSON hợp lệ")
		}
		name := strings.ToLower(strings.TrimSpace(reqPayload.Name))
		if name == "" || s.scriptAllowlist[name] == "" {
			return fmt.Errorf("script không nằm trong allowlist")
		}
		if len(reqPayload.Args) > 20 {
			return fmt.Errorf("script chỉ được nhận tối đa 20 tham số")
		}
		for _, arg := range reqPayload.Args {
			if strings.ContainsAny(arg, "\x00\r\n") || len([]rune(arg)) > 256 {
				return fmt.Errorf("tham số script không hợp lệ")
			}
		}
		if strings.TrimSpace(reqPayload.Timeout) != "" {
			parsed, err := time.ParseDuration(strings.TrimSpace(reqPayload.Timeout))
			if err != nil || parsed <= 0 || parsed > 10*time.Minute {
				return fmt.Errorf("timeout script phải hợp lệ và không quá 10m")
			}
		}
	default:
		return fmt.Errorf("runner không được hỗ trợ")
	}
	return nil
}

func normalizeScriptAllowlist(values map[string]string) map[string]string {
	result := map[string]string{}
	for name, path := range values {
		name = strings.ToLower(strings.TrimSpace(name))
		path = strings.TrimSpace(path)
		if name == "" || path == "" {
			continue
		}
		result[name] = path
	}
	return result
}

func allowedHTTPMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func cleanupRetention(task string) time.Duration {
	switch task {
	case "cleanup_expired_sessions":
		return 7 * 24 * time.Hour
	case "cleanup_dead_outbox":
		return 30 * 24 * time.Hour
	case "cleanup_failed_uploads":
		return 24 * time.Hour
	case "cleanup_old_presence":
		return 7 * 24 * time.Hour
	default:
		return 30 * 24 * time.Hour
	}
}

func truncateRunLog(value string) string {
	value = strings.TrimSpace(value)
	if len([]rune(value)) <= maxRunLogLength {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxRunLogLength])
}
