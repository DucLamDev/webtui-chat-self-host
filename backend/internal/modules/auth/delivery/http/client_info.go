package http

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

func clientIP(c *gin.Context) string {
	for _, header := range []string{"CF-Connecting-IP", "X-Real-IP", "X-Forwarded-For"} {
		value := strings.TrimSpace(c.GetHeader(header))
		if value == "" {
			continue
		}
		if header == "X-Forwarded-For" {
			value = strings.TrimSpace(strings.Split(value, ",")[0])
		}
		if net.ParseIP(value) != nil {
			return value
		}
	}

	value := strings.TrimSpace(c.ClientIP())
	if net.ParseIP(value) != nil {
		return value
	}
	return ""
}
