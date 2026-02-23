package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

// CacheControl sets the Cache-Control header for responses, usually static assets.
func CacheControl(maxAgeSeconds int) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", maxAgeSeconds))
		c.Next()
	}
}
