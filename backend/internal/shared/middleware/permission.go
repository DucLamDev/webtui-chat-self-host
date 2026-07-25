package middleware

import (
	"context"
	"net/http"

	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type PermissionChecker interface {
	HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error)
}

func RequireWorkspacePermission(checker PermissionChecker, permissionCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := CurrentUserID(c)
		if userID == "" {
			response.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "Bạn cần đăng nhập để tiếp tục.", nil)
			c.Abort()
			return
		}

		workspaceID := c.Param("workspace_id")
		if workspaceID == "" {
			workspaceID = c.Query("workspace_id")
		}
		if workspaceID == "" {
			response.Fail(c, http.StatusBadRequest, "WORKSPACE_REQUIRED", "Thiếu workspace_id để kiểm tra quyền.", nil)
			c.Abort()
			return
		}

		allowed, err := checker.HasWorkspacePermission(c.Request.Context(), userID, workspaceID, permissionCode)
		if err != nil {
			response.Error(c, err)
			c.Abort()
			return
		}
		if !allowed {
			response.Fail(c, http.StatusForbidden, "PERMISSION_DENIED", "Bạn không có quyền thực hiện thao tác này.", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}
