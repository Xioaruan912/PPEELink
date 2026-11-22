package middlewares

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders attaches baseline browser security headers to every response
// and prevents intermediaries/browsers from caching authenticated API payloads.
func SecurityHeaders(c *gin.Context) {
	h := c.Writer.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "same-origin")
	h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	if strings.HasPrefix(c.Request.URL.Path, "/api/") {
		h.Set("Cache-Control", "no-store")
	}
	c.Next()
}
