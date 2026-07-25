package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	orderapp "github.com/duclamdev/application-chat/backend/internal/modules/order/application"
)

const maxErrorBodyBytes = 4096

type Config struct {
	BaseURL        string
	InternalAPIKey string
	QuickOrderKey  string
	Timeout        time.Duration
}

type Client struct {
	baseURL        *url.URL
	internalAPIKey string
	quickOrderKey  string
	httpClient     *http.Client
}

func New(config Config) *Client {
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	parsed, _ := url.Parse(strings.TrimRight(strings.TrimSpace(config.BaseURL), "/"))
	return &Client{
		baseURL:        parsed,
		internalAPIKey: strings.TrimSpace(config.InternalAPIKey),
		quickOrderKey:  strings.TrimSpace(config.QuickOrderKey),
		httpClient:     &http.Client{Timeout: timeout},
	}
}

func (c *Client) QuickOrderConfigured() bool {
	return c != nil && c.baseURL != nil && c.baseURL.Scheme != "" && c.baseURL.Host != "" && c.quickOrderKey != ""
}

func (c *Client) Configured() bool {
	return c != nil && c.baseURL != nil && c.baseURL.Scheme != "" && c.baseURL.Host != "" && c.internalAPIKey != ""
}

func (c *Client) WalletBalance(ctx context.Context, input orderapp.UserLookupRequest) (orderapp.WalletBalanceEnvelope, error) {
	var output orderapp.WalletBalanceEnvelope
	err := c.post(ctx, "/internal/wallet/balance", input, &output)
	return output, err
}

func (c *Client) CreateDepositQR(ctx context.Context, input orderapp.WalletDepositQRRequest) (orderapp.WalletDepositQREnvelope, error) {
	var output orderapp.WalletDepositQREnvelope
	err := c.post(ctx, "/internal/wallet/deposit-qr", input, &output)
	return output, err
}

func (c *Client) ServicesExpiring(ctx context.Context, input orderapp.ServicesExpiringRequest) (orderapp.ServicesExpiringEnvelope, error) {
	var output orderapp.ServicesExpiringEnvelope
	err := c.post(ctx, "/internal/services/expiring", input, &output)
	return output, err
}

func (c *Client) RenewService(ctx context.Context, input orderapp.RenewServiceRequest) (orderapp.RenewServiceEnvelope, error) {
	var output orderapp.RenewServiceEnvelope
	err := c.post(ctx, "/internal/services/renew", input, &output)
	return output, err
}

func (c *Client) CreateOrderPaymentQR(ctx context.Context, input orderapp.OrderPaymentQRRequest) (orderapp.OrderPaymentQREnvelope, error) {
	var output orderapp.OrderPaymentQREnvelope
	err := c.postWithKey(ctx, "/quick-order/payment/qr", "X-Quick-Order-Key", c.quickOrderKey, input, &output)
	return output, err
}

func (c *Client) post(ctx context.Context, path string, input any, output any) error {
	if !c.Configured() {
		return fmt.Errorf("order API client is not configured")
	}
	return c.postWithKey(ctx, path, "X-API-Key", c.internalAPIKey, input, output)
}

func (c *Client) postWithKey(ctx context.Context, path string, header string, key string, input any, output any) error {
	if c == nil || c.baseURL == nil || c.baseURL.Scheme == "" || c.baseURL.Host == "" || strings.TrimSpace(key) == "" {
		return fmt.Errorf("order API client is not configured")
	}
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	endpoint := c.resolve(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(header, key)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return parseUpstreamError(resp.StatusCode, responseBody)
	}
	if err := json.NewDecoder(resp.Body).Decode(output); err != nil {
		return fmt.Errorf("decode order API response: %w", err)
	}
	return nil
}

func parseUpstreamError(statusCode int, body []byte) error {
	var payload struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &payload)
	message := strings.TrimSpace(payload.Message)
	if message == "" {
		message = http.StatusText(statusCode)
	}
	return &orderapp.UpstreamError{
		StatusCode: statusCode,
		Message:    message,
	}
}

func (c *Client) resolve(path string) string {
	base := *c.baseURL
	base.Path = strings.TrimRight(base.Path, "/") + "/" + strings.TrimLeft(path, "/")
	return base.String()
}
