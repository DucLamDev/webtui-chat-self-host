package minio

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/duclamdev/application-chat/backend/internal/config"
	"github.com/duclamdev/application-chat/backend/internal/platform/storage"
)

const unsignedPayload = "UNSIGNED-PAYLOAD"

type Client struct {
	endpoint        *url.URL
	region          string
	bucket          string
	accessKeyID     string
	secretAccessKey string
	httpClient      *http.Client
}

func New(cfg config.StorageConfig) (*Client, error) {
	endpoint, err := url.Parse(strings.TrimRight(cfg.Endpoint, "/"))
	if err != nil {
		return nil, fmt.Errorf("phân tích endpoint MinIO: %w", err)
	}
	if endpoint.Scheme == "" || endpoint.Host == "" {
		return nil, errors.New("endpoint MinIO không hợp lệ")
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, errors.New("bucket MinIO là bắt buộc")
	}
	if strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.SecretAccessKey) == "" {
		return nil, errors.New("access key và secret key MinIO là bắt buộc")
	}
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "us-east-1"
	}
	return &Client{
		endpoint:        endpoint,
		region:          region,
		bucket:          strings.TrimSpace(cfg.Bucket),
		accessKeyID:     strings.TrimSpace(cfg.AccessKeyID),
		secretAccessKey: strings.TrimSpace(cfg.SecretAccessKey),
		httpClient:      &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (c *Client) Put(ctx context.Context, input storage.PutObjectInput) (storage.ObjectInfo, error) {
	if strings.TrimSpace(input.Key) == "" {
		return storage.ObjectInfo{}, errors.New("khóa đối tượng lưu trữ là bắt buộc")
	}
	req, err := c.newRequest(ctx, http.MethodPut, input.Key, input.Body)
	if err != nil {
		return storage.ObjectInfo{}, err
	}
	req.Header.Set("Content-Type", input.ContentType)
	if input.Size >= 0 {
		req.ContentLength = input.Size
	}
	if err := c.do(req, http.StatusOK); err != nil {
		return storage.ObjectInfo{}, err
	}
	return storage.ObjectInfo{
		Key:         input.Key,
		ContentType: input.ContentType,
		Size:        input.Size,
	}, nil
}

func (c *Client) Get(ctx context.Context, key string) (*storage.GetObjectOutput, error) {
	req, err := c.newRequest(ctx, http.MethodGet, key, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, c.statusError(resp)
	}
	return &storage.GetObjectOutput{
		Info: storage.ObjectInfo{
			Key:         key,
			ContentType: resp.Header.Get("Content-Type"),
			Size:        resp.ContentLength,
		},
		Body: resp.Body,
	}, nil
}

func (c *Client) Delete(ctx context.Context, key string) error {
	req, err := c.newRequest(ctx, http.MethodDelete, key, nil)
	if err != nil {
		return err
	}
	return c.do(req, http.StatusNoContent)
}

func (c *Client) Health(ctx context.Context) error {
	req, err := c.newBucketRequest(ctx, http.MethodHead)
	if err != nil {
		return err
	}
	return c.do(req, http.StatusOK)
}

func (c *Client) newRequest(ctx context.Context, method string, key string, body io.Reader) (*http.Request, error) {
	key = strings.TrimPrefix(strings.TrimSpace(key), "/")
	if key == "" || strings.HasPrefix(key, "../") || strings.Contains(key, "/../") {
		return nil, errors.New("khóa đối tượng lưu trữ không hợp lệ")
	}
	rawPath := c.objectPathRaw(key)
	canonicalPath := c.objectPath(key)
	reqURL := *c.endpoint
	reqURL.Path = rawPath
	reqURL.RawPath = canonicalPath
	req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), body)
	if err != nil {
		return nil, err
	}
	c.sign(req, canonicalPath)
	return req, nil
}

func (c *Client) newBucketRequest(ctx context.Context, method string) (*http.Request, error) {
	rawPath := c.bucketPathRaw()
	canonicalPath := c.bucketPath()
	reqURL := *c.endpoint
	reqURL.Path = rawPath
	reqURL.RawPath = canonicalPath
	req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), nil)
	if err != nil {
		return nil, err
	}
	c.sign(req, canonicalPath)
	return req, nil
}

func (c *Client) do(req *http.Request, expectedStatus int) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == expectedStatus {
		return nil
	}
	return c.statusError(resp)
}

func (c *Client) statusError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = resp.Status
	}
	return fmt.Errorf("MinIO trả về trạng thái %d: %s", resp.StatusCode, message)
}

func (c *Client) sign(req *http.Request, canonicalPath string) {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", unsignedPayload)

	canonicalHeaders := "host:" + req.URL.Host + "\n" +
		"x-amz-content-sha256:" + unsignedPayload + "\n" +
		"x-amz-date:" + amzDate + "\n"
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalPath,
		"",
		canonicalHeaders,
		signedHeaders,
		unsignedPayload,
	}, "\n")

	scope := dateStamp + "/" + c.region + "/s3/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")
	signature := hex.EncodeToString(hmacSHA256(c.signingKey(dateStamp), []byte(stringToSign)))
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+c.accessKeyID+"/"+scope+", SignedHeaders="+signedHeaders+", Signature="+signature)
}

func (c *Client) signingKey(dateStamp string) []byte {
	dateKey := hmacSHA256([]byte("AWS4"+c.secretAccessKey), []byte(dateStamp))
	regionKey := hmacSHA256(dateKey, []byte(c.region))
	serviceKey := hmacSHA256(regionKey, []byte("s3"))
	return hmacSHA256(serviceKey, []byte("aws4_request"))
}

func (c *Client) bucketPath() string {
	return joinPath(c.endpoint.Path, pathEscape(c.bucket))
}

func (c *Client) objectPath(key string) string {
	return joinPath(c.endpoint.Path, pathEscape(c.bucket), escapeKey(key))
}

func (c *Client) bucketPathRaw() string {
	return joinPath(c.endpoint.Path, c.bucket)
}

func (c *Client) objectPathRaw(key string) string {
	return joinPath(c.endpoint.Path, c.bucket, key)
}

func joinPath(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(part, "/")
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return "/" + strings.Join(cleaned, "/")
}

func escapeKey(key string) string {
	segments := strings.Split(key, "/")
	for index, segment := range segments {
		segments[index] = pathEscape(segment)
	}
	return strings.Join(segments, "/")
}

func pathEscape(value string) string {
	return strings.ReplaceAll(url.PathEscape(value), "+", "%20")
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}
