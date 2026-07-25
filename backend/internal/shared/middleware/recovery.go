package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("Đã khôi phục sau lỗi nghiêm trọng",
					"error", recovered,
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
					"request_id", GetRequestID(c),
					"stack", string(debug.Stack()),
				)

				response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Hệ thống đang bận, vui lòng thử lại sau.", nil)
				c.Abort()
			}
		}()

		c.Next()
	}
}
