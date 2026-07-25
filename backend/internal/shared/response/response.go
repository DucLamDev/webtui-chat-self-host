package response

import (
	"context"
	stderrors "errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/puddle/v2"
)

type Body struct {
	Success   bool       `json:"success"`
	Data      any        `json:"data,omitempty"`
	Error     *ErrorBody `json:"error,omitempty"`
	Meta      any        `json:"meta,omitempty"`
	RequestID string     `json:"request_id,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
}

type ErrorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func OK(c *gin.Context, status int, data any) {
	c.JSON(status, Body{
		Success:   true,
		Data:      data,
		RequestID: requestID(c),
		Timestamp: time.Now().UTC(),
	})
}

func OKWithMeta(c *gin.Context, status int, data any, meta any) {
	c.JSON(status, Body{
		Success:   true,
		Data:      data,
		Meta:      meta,
		RequestID: requestID(c),
		Timestamp: time.Now().UTC(),
	})
}

func Created(c *gin.Context, data any) {
	OK(c, http.StatusCreated, data)
}

func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func Fail(c *gin.Context, status int, code string, message string, details map[string]any) {
	c.JSON(status, Body{
		Success: false,
		Error: &ErrorBody{
			Code:    code,
			Message: message,
			Details: details,
		},
		RequestID: requestID(c),
		Timestamp: time.Now().UTC(),
	})
}

func Error(c *gin.Context, err error) {
	var appErr *apperrors.AppError
	if stderrors.As(err, &appErr) {
		if appErr.Status >= http.StatusInternalServerError {
			slog.Error("Yêu cầu API thất bại với lỗi ứng dụng",
				"error", err,
				"code", appErr.Code,
				"status", appErr.Status,
				"method", requestMethod(c),
				"path", requestPath(c),
				"request_id", requestID(c),
			)
		}
		Fail(c, appErr.Status, appErr.Code, appErr.Message, appErr.Details)
		return
	}

	if stderrors.Is(err, context.Canceled) {
		Fail(c, http.StatusRequestTimeout, "REQUEST_CANCELLED", "Yêu cầu đã bị hủy trước khi hoàn tất.", nil)
		return
	}
	if stderrors.Is(err, context.DeadlineExceeded) {
		Fail(c, http.StatusGatewayTimeout, "REQUEST_TIMEOUT", "Yêu cầu xử lý quá thời gian cho phép, vui lòng thử lại.", nil)
		return
	}
	if stderrors.Is(err, pgx.ErrNoRows) {
		Fail(c, http.StatusNotFound, "RESOURCE_NOT_FOUND", "Không tìm thấy dữ liệu yêu cầu.", nil)
		return
	}

	var pgErr *pgconn.PgError
	if stderrors.As(err, &pgErr) {
		if writePostgresError(c, pgErr) {
			return
		}
	}
	if writePostgresTransportError(c, err) {
		return
	}

	slog.Error("Yêu cầu API thất bại ngoài dự kiến",
		"error", err,
		"method", requestMethod(c),
		"path", requestPath(c),
		"request_id", requestID(c),
	)
	Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Hệ thống đang bận, vui lòng thử lại sau.", nil)
}

func writePostgresError(c *gin.Context, err *pgconn.PgError) bool {
	if err.Code == "P0001" && strings.HasPrefix(err.Message, "ZONE_QUOTA_EXCEEDED:") {
		resource := strings.TrimPrefix(err.Message, "ZONE_QUOTA_EXCEEDED:")
		Fail(
			c,
			http.StatusConflict,
			"ZONE_QUOTA_EXCEEDED",
			"Zone da dat gioi han tai nguyen.",
			map[string]any{"resource": resource},
		)
		return true
	}
	switch err.Code {
	case "22P02":
		Fail(c, http.StatusBadRequest, "INVALID_IDENTIFIER", "Mã định danh trong yêu cầu không hợp lệ.", nil)
	case "23502", "23514":
		Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu gửi lên không hợp lệ.", nil)
	case "23503":
		Fail(c, http.StatusConflict, "RELATED_RESOURCE_CONFLICT", "Dữ liệu liên quan không tồn tại hoặc đang được sử dụng.", nil)
	case "23505":
		Fail(c, http.StatusConflict, "RESOURCE_ALREADY_EXISTS", "Dữ liệu đã tồn tại, vui lòng tải lại trang.", nil)
	case "40001", "40P01", "55P03":
		c.Header("Retry-After", "1")
		Fail(c, http.StatusServiceUnavailable, "DATABASE_RETRY_REQUIRED", "Dữ liệu vừa được thay đổi đồng thời, vui lòng thử lại.", nil)
	case "21000":
		Fail(c, http.StatusConflict, "DATA_CONSISTENCY_CONFLICT", "Du lieu hoi thoai dang khong nhat quan, vui long tai lai trang va thu lai.", nil)
	case "57014":
		c.Header("Retry-After", "1")
		Fail(c, http.StatusGatewayTimeout, "DATABASE_TIMEOUT", "Truy vấn dữ liệu quá thời gian cho phép, vui lòng thử lại.", nil)
	case "57P01", "57P02", "57P03", "57P04":
		c.Header("Retry-After", "2")
		Fail(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "Cơ sở dữ liệu tạm thời không sẵn sàng, vui lòng thử lại sau vài giây.", nil)
	default:
		if strings.HasPrefix(err.Code, "08") || strings.HasPrefix(err.Code, "53") || strings.HasPrefix(err.Code, "58") {
			c.Header("Retry-After", "2")
			Fail(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "Cơ sở dữ liệu tạm thời không sẵn sàng, vui lòng thử lại sau vài giây.", nil)
		} else if strings.HasPrefix(err.Code, "42") {
			Fail(c, http.StatusInternalServerError, "DATABASE_SCHEMA_MISMATCH", "Co so du lieu chua duoc cap nhat dung phien ban. Vui long chay migration va thu lai.", nil)
		} else {
			Fail(c, http.StatusInternalServerError, "DATABASE_ERROR", "Co so du lieu tra ve loi khi xu ly yeu cau. Vui long kiem tra log theo ma request.", nil)
		}
	}

	slog.Warn("Yêu cầu API gặp lỗi PostgreSQL đã được phân loại",
		"sqlstate", err.Code,
		"constraint", err.ConstraintName,
		"method", requestMethod(c),
		"path", requestPath(c),
		"request_id", requestID(c),
	)
	return true
}

func writePostgresTransportError(c *gin.Context, err error) bool {
	var connectErr *pgconn.ConnectError
	switch {
	case pgconn.Timeout(err):
		c.Header("Retry-After", "1")
		Fail(c, http.StatusGatewayTimeout, "DATABASE_TIMEOUT", "Kết nối cơ sở dữ liệu quá thời gian cho phép, vui lòng thử lại.", nil)
	case stderrors.As(err, &connectErr),
		stderrors.Is(err, pgconn.ErrConnClosed),
		stderrors.Is(err, puddle.ErrClosedPool),
		stderrors.Is(err, puddle.ErrNotAvailable),
		pgconn.SafeToRetry(err):
		c.Header("Retry-After", "2")
		Fail(c, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "Cơ sở dữ liệu tạm thời không sẵn sàng, vui lòng thử lại sau vài giây.", nil)
	default:
		return false
	}

	slog.Warn("Yêu cầu API gặp lỗi kết nối PostgreSQL",
		"error", err,
		"method", requestMethod(c),
		"path", requestPath(c),
		"request_id", requestID(c),
	)
	return true
}

func requestMethod(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	return c.Request.Method
}

func requestPath(c *gin.Context) string {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return ""
	}
	return c.Request.URL.Path
}

func requestID(c *gin.Context) string {
	value, ok := c.Get("request_id")
	if !ok {
		return ""
	}

	requestID, _ := value.(string)
	return requestID
}
