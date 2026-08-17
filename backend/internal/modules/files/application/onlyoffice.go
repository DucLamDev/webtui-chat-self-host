package application

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	filesdomain "github.com/duclamdev/application-chat/backend/internal/modules/files/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

const (
	defaultOnlyOfficeSessionTTL = 24 * time.Hour
	onlyOfficeDownloadPurpose   = "download"
	onlyOfficeCallbackPurpose   = "callback"
)

var onlyOfficeDocumentKeyPattern = regexp.MustCompile(`[^A-Za-z0-9._=-]`)

type OnlyOfficeHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type OnlyOfficeOptions struct {
	Enabled       bool
	PublicURL     string
	InternalURL   string
	APIBaseURL    string
	JWTSecret     string
	SessionSecret string
	SessionTTL    time.Duration
	HTTPClient    OnlyOfficeHTTPClient
}

type OnlyOfficeSessionInput struct {
	ActorUserID string
	WorkspaceID string
	FileID      string
	UserName    string
}

type OnlyOfficeEditorSessionDTO struct {
	Enabled           bool           `json:"enabled"`
	SessionID         string         `json:"session_id"`
	DocumentServerURL string         `json:"document_server_url"`
	ScriptURL         string         `json:"script_url"`
	Config            map[string]any `json:"config"`
	ExpiresAt         string         `json:"expires_at"`
}

type OnlyOfficeCallbackPayload struct {
	Key      string   `json:"key"`
	Status   int      `json:"status"`
	URL      string   `json:"url"`
	FileType string   `json:"filetype"`
	Users    []string `json:"users"`
}

type OnlyOfficeCallbackInput struct {
	WorkspaceID string
	FileID      string
	Token       string
	Payload     OnlyOfficeCallbackPayload
}

type OnlyOfficeCallbackResult struct {
	Saved      bool
	Version    *VersionDTO
	NoopReason string
}

type onlyOfficeSessionClaims struct {
	WorkspaceID string `json:"workspace_id"`
	FileID      string `json:"file_id"`
	ActorUserID string `json:"actor_user_id"`
	Purpose     string `json:"purpose"`
	ExpiresAt   int64  `json:"expires_at"`
	Nonce       string `json:"nonce"`
}

func (o OnlyOfficeOptions) normalized() OnlyOfficeOptions {
	o.PublicURL = strings.TrimRight(strings.TrimSpace(o.PublicURL), "/")
	o.InternalURL = strings.TrimRight(strings.TrimSpace(o.InternalURL), "/")
	o.APIBaseURL = strings.TrimRight(strings.TrimSpace(o.APIBaseURL), "/")
	o.JWTSecret = strings.TrimSpace(o.JWTSecret)
	o.SessionSecret = strings.TrimSpace(o.SessionSecret)
	if o.SessionTTL <= 0 {
		o.SessionTTL = defaultOnlyOfficeSessionTTL
	}
	if o.HTTPClient == nil {
		o.HTTPClient = &http.Client{Timeout: 60 * time.Second}
	}
	return o
}

func (s *Service) CreateOnlyOfficeSession(ctx context.Context, input OnlyOfficeSessionInput) (OnlyOfficeEditorSessionDTO, error) {
	options := s.onlyOffice.normalized()
	if !options.Enabled {
		return OnlyOfficeEditorSessionDTO{}, apperrors.ServiceUnavailable("ONLYOFFICE_DISABLED", "ONLYOFFICE chua duoc cau hinh cho he thong nay.")
	}
	if options.PublicURL == "" || options.APIBaseURL == "" || options.JWTSecret == "" || options.SessionSecret == "" {
		return OnlyOfficeEditorSessionDTO{}, apperrors.ServiceUnavailable("ONLYOFFICE_NOT_CONFIGURED", "Cau hinh ONLYOFFICE chua day du.")
	}
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "file.upload"); err != nil {
		return OnlyOfficeEditorSessionDTO{}, err
	}
	if err := s.ensureFileAccess(ctx, input.ActorUserID, input.WorkspaceID, input.FileID); err != nil {
		return OnlyOfficeEditorSessionDTO{}, err
	}
	file, err := s.repo.FindFile(ctx, strings.TrimSpace(input.WorkspaceID), strings.TrimSpace(input.FileID))
	if err != nil {
		return OnlyOfficeEditorSessionDTO{}, mapFileError(err)
	}
	fileType := officeFileType(file.OriginalName, file.MimeType)
	documentType := onlyOfficeDocumentType(fileType)
	if documentType == "" {
		return OnlyOfficeEditorSessionDTO{}, apperrors.BadRequest("ONLYOFFICE_UNSUPPORTED_FILE", "ONLYOFFICE chi ho tro sua Word va Excel o luong nay.")
	}

	expiresAt := s.now().Add(options.SessionTTL)
	downloadToken, err := s.signOnlyOfficeSessionToken(options, onlyOfficeSessionClaims{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		FileID:      strings.TrimSpace(input.FileID),
		ActorUserID: strings.TrimSpace(input.ActorUserID),
		Purpose:     onlyOfficeDownloadPurpose,
		ExpiresAt:   expiresAt.Unix(),
		Nonce:       randomTokenNonce(),
	})
	if err != nil {
		return OnlyOfficeEditorSessionDTO{}, err
	}
	callbackToken, err := s.signOnlyOfficeSessionToken(options, onlyOfficeSessionClaims{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		FileID:      strings.TrimSpace(input.FileID),
		ActorUserID: strings.TrimSpace(input.ActorUserID),
		Purpose:     onlyOfficeCallbackPurpose,
		ExpiresAt:   expiresAt.Unix(),
		Nonce:       randomTokenNonce(),
	})
	if err != nil {
		return OnlyOfficeEditorSessionDTO{}, err
	}

	basePath := options.APIBaseURL + "/api/v1/office/workspaces/" + url.PathEscape(input.WorkspaceID) + "/files/" + url.PathEscape(input.FileID)
	downloadURL := basePath + "/download?token=" + url.QueryEscape(downloadToken)
	callbackURL := basePath + "/callback?token=" + url.QueryEscape(callbackToken)
	config := map[string]any{
		"documentType": documentType,
		"type":         "desktop",
		"height":       "100%",
		"width":        "100%",
		"document": map[string]any{
			"fileType": fileType,
			"key":      onlyOfficeDocumentKey(file),
			"title":    file.OriginalName,
			"url":      downloadURL,
			"permissions": map[string]any{
				"download": true,
				"edit":     true,
				"print":    true,
				"review":   true,
			},
		},
		"editorConfig": map[string]any{
			"callbackUrl": callbackURL,
			"lang":        "vi",
			"mode":        "edit",
			"user": map[string]any{
				"id":   strings.TrimSpace(input.ActorUserID),
				"name": firstNonEmptyString(strings.TrimSpace(input.UserName), strings.TrimSpace(input.ActorUserID)),
			},
			"customization": map[string]any{
				"autosave":  true,
				"forcesave": true,
			},
		},
	}
	token, err := signOnlyOfficeJWT(config, options.JWTSecret)
	if err != nil {
		return OnlyOfficeEditorSessionDTO{}, err
	}
	config["token"] = token

	return OnlyOfficeEditorSessionDTO{
		Enabled:           true,
		SessionID:         onlyOfficeDocumentKey(file),
		DocumentServerURL: options.PublicURL,
		ScriptURL:         options.PublicURL + "/web-apps/apps/api/documents/api.js",
		Config:            config,
		ExpiresAt:         expiresAt.UTC().Format(time.RFC3339),
	}, nil
}

func (s *Service) DownloadOnlyOfficeSource(ctx context.Context, workspaceID string, fileID string, token string) (DownloadDTO, error) {
	_, err := s.verifyOnlyOfficeSessionToken(workspaceID, fileID, onlyOfficeDownloadPurpose, token)
	if err != nil {
		return DownloadDTO{}, err
	}
	file, err := s.repo.FindFile(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(fileID))
	if err != nil {
		return DownloadDTO{}, mapFileError(err)
	}
	if onlyOfficeDocumentType(officeFileType(file.OriginalName, file.MimeType)) == "" {
		return DownloadDTO{}, apperrors.BadRequest("ONLYOFFICE_UNSUPPORTED_FILE", "File khong duoc ho tro sua bang ONLYOFFICE.")
	}
	storageLocation, err := s.storageForExistingFile(ctx, workspaceID, file.StorageProvider, file.Bucket)
	if err != nil {
		return DownloadDTO{}, err
	}
	object, err := storageLocation.Store.Get(ctx, file.ObjectKey)
	if err != nil {
		return DownloadDTO{}, err
	}
	return DownloadDTO{
		File:          toFileDTO(file),
		Body:          object.Body,
		ContentLength: file.ByteSize,
	}, nil
}

func (s *Service) HandleOnlyOfficeCallback(ctx context.Context, input OnlyOfficeCallbackInput) (OnlyOfficeCallbackResult, error) {
	claims, err := s.verifyOnlyOfficeSessionToken(input.WorkspaceID, input.FileID, onlyOfficeCallbackPurpose, input.Token)
	if err != nil {
		return OnlyOfficeCallbackResult{}, err
	}
	if input.Payload.Status != 2 && input.Payload.Status != 6 {
		return OnlyOfficeCallbackResult{NoopReason: "status_" + strconv.Itoa(input.Payload.Status)}, nil
	}
	if strings.TrimSpace(input.Payload.URL) == "" {
		return OnlyOfficeCallbackResult{}, apperrors.BadRequest("ONLYOFFICE_CALLBACK_MISSING_URL", "ONLYOFFICE khong gui URL file da sua.")
	}
	file, err := s.repo.FindFile(ctx, strings.TrimSpace(input.WorkspaceID), strings.TrimSpace(input.FileID))
	if err != nil {
		return OnlyOfficeCallbackResult{}, mapFileError(err)
	}
	data, contentType, err := s.fetchOnlyOfficeResult(ctx, input.Payload.URL)
	if err != nil {
		return OnlyOfficeCallbackResult{}, err
	}
	checksum := sha256.Sum256(data)
	checksumHex := hex.EncodeToString(checksum[:])
	if file.ChecksumSHA256 != nil && strings.EqualFold(strings.TrimSpace(*file.ChecksumSHA256), checksumHex) {
		return OnlyOfficeCallbackResult{NoopReason: "same_checksum"}, nil
	}
	fileType := normalizeOfficeFileType(input.Payload.FileType)
	if fileType == "" {
		fileType = officeFileType("", contentType)
	}
	if fileType == "" {
		fileType = officeFileType(file.OriginalName, file.MimeType)
	}
	mimeType := mimeTypeForOfficeFileType(fileType)
	if mimeType == "" {
		mimeType = firstNonEmptyString(strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0])), file.MimeType)
	}
	version, err := s.createVersion(ctx, UploadInput{
		ActorUserID:  claims.ActorUserID,
		WorkspaceID:  input.WorkspaceID,
		OriginalName: filenameWithExtension(file.OriginalName, fileType),
		MimeType:     mimeType,
		Size:         int64(len(data)),
		Body:         bytes.NewReader(data),
	}, input.FileID, false)
	if err != nil {
		return OnlyOfficeCallbackResult{}, err
	}
	return OnlyOfficeCallbackResult{Saved: true, Version: &version}, nil
}

func (s *Service) fetchOnlyOfficeResult(ctx context.Context, rawURL string) ([]byte, string, error) {
	fetchURL, err := s.onlyOfficeResultURL(rawURL)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fetchURL, nil)
	if err != nil {
		return nil, "", apperrors.BadRequest("ONLYOFFICE_INVALID_RESULT_URL", "URL ket qua tu ONLYOFFICE khong hop le.")
	}
	resp, err := s.onlyOffice.normalized().HTTPClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", apperrors.New("ONLYOFFICE_RESULT_DOWNLOAD_FAILED", "Khong tai duoc file da sua tu ONLYOFFICE.", http.StatusBadGateway)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxUploadSize+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > maxUploadSize {
		return nil, "", apperrors.BadRequest("VALIDATION_ERROR", "File ONLYOFFICE tra ve vuot qua 100MB.")
	}
	return data, resp.Header.Get("Content-Type"), nil
}

func (s *Service) onlyOfficeResultURL(rawURL string) (string, error) {
	options := s.onlyOffice.normalized()
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", apperrors.BadRequest("ONLYOFFICE_INVALID_RESULT_URL", "URL ket qua tu ONLYOFFICE khong hop le.")
	}
	if onlyOfficeURLHasBase(rawURL, options.PublicURL) {
		if options.InternalURL != "" {
			return options.InternalURL + strings.TrimPrefix(rawURL, options.PublicURL), nil
		}
		return rawURL, nil
	}
	if onlyOfficeURLHasBase(rawURL, options.InternalURL) {
		return rawURL, nil
	}
	return "", apperrors.BadRequest("ONLYOFFICE_INVALID_RESULT_URL", "URL ket qua tu ONLYOFFICE khong dung Document Server da cau hinh.")
}

func onlyOfficeURLHasBase(rawURL string, baseURL string) bool {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return false
	}
	return rawURL == baseURL || strings.HasPrefix(rawURL, baseURL+"/")
}

func (s *Service) signOnlyOfficeSessionToken(options OnlyOfficeOptions, claims onlyOfficeSessionClaims) (string, error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(options.SessionSecret))
	_, _ = mac.Write([]byte(encoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encoded + "." + signature, nil
}

func (s *Service) verifyOnlyOfficeSessionToken(workspaceID string, fileID string, purpose string, token string) (onlyOfficeSessionClaims, error) {
	options := s.onlyOffice.normalized()
	if !options.Enabled || options.SessionSecret == "" {
		return onlyOfficeSessionClaims{}, apperrors.ServiceUnavailable("ONLYOFFICE_NOT_CONFIGURED", "Cau hinh ONLYOFFICE chua day du.")
	}
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return onlyOfficeSessionClaims{}, apperrors.Unauthorized("Token ONLYOFFICE khong hop le.")
	}
	mac := hmac.New(sha256.New, []byte(options.SessionSecret))
	_, _ = mac.Write([]byte(parts[0]))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return onlyOfficeSessionClaims{}, apperrors.Unauthorized("Token ONLYOFFICE khong hop le.")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return onlyOfficeSessionClaims{}, apperrors.Unauthorized("Token ONLYOFFICE khong hop le.")
	}
	var claims onlyOfficeSessionClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return onlyOfficeSessionClaims{}, apperrors.Unauthorized("Token ONLYOFFICE khong hop le.")
	}
	if claims.ExpiresAt <= s.now().Unix() {
		return onlyOfficeSessionClaims{}, apperrors.Unauthorized("Token ONLYOFFICE da het han.")
	}
	if claims.Purpose != purpose ||
		strings.TrimSpace(claims.WorkspaceID) != strings.TrimSpace(workspaceID) ||
		strings.TrimSpace(claims.FileID) != strings.TrimSpace(fileID) {
		return onlyOfficeSessionClaims{}, apperrors.Forbidden("Token ONLYOFFICE khong khop file.")
	}
	return claims, nil
}

func signOnlyOfficeJWT(payload any, secret string) (string, error) {
	header, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encodedHeader := base64.RawURLEncoding.EncodeToString(header)
	encodedBody := base64.RawURLEncoding.EncodeToString(body)
	unsigned := encodedHeader + "." + encodedBody
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func officeFileType(name string, mimeType string) string {
	extension := normalizeOfficeFileType(strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), "."))
	if extension != "" {
		return extension
	}
	switch strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0])) {
	case "application/msword":
		return "doc"
	case "application/vnd.ms-excel":
		return "xls"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return "docx"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return "xlsx"
	default:
		return ""
	}
}

func normalizeOfficeFileType(value string) string {
	switch strings.ToLower(strings.TrimPrefix(strings.TrimSpace(value), ".")) {
	case "doc", "docx", "odt", "rtf", "txt":
		return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(value), "."))
	case "xls", "xlsx", "ods", "csv":
		return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(value), "."))
	default:
		return ""
	}
}

func onlyOfficeDocumentType(fileType string) string {
	switch fileType {
	case "doc", "docx", "odt", "rtf", "txt":
		return "word"
	case "xls", "xlsx", "ods", "csv":
		return "cell"
	default:
		return ""
	}
}

func mimeTypeForOfficeFileType(fileType string) string {
	switch fileType {
	case "doc":
		return "application/msword"
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "odt":
		return "application/vnd.oasis.opendocument.text"
	case "rtf":
		return "application/rtf"
	case "txt":
		return "text/plain"
	case "xls":
		return "application/vnd.ms-excel"
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "ods":
		return "application/vnd.oasis.opendocument.spreadsheet"
	case "csv":
		return "text/csv"
	default:
		return ""
	}
}

func filenameWithExtension(name string, extension string) string {
	extension = normalizeOfficeFileType(extension)
	if extension == "" {
		return name
	}
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if strings.TrimSpace(base) == "" {
		base = "document"
	}
	return base + "." + extension
}

func onlyOfficeDocumentKey(file filesdomain.File) string {
	checksum := ""
	if file.ChecksumSHA256 != nil {
		checksum = strings.TrimSpace(*file.ChecksumSHA256)
	}
	if checksum == "" {
		checksum = strconv.FormatInt(file.UpdatedAt.Unix(), 10)
	}
	key := file.ID + "-" + checksum
	key = onlyOfficeDocumentKeyPattern.ReplaceAllString(key, "_")
	if len(key) > 120 {
		key = key[:120]
	}
	return key
}

func randomTokenNonce() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(raw[:])
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
