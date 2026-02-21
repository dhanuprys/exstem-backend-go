package response

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ContextKeyRequestID is the Gin context key for the request ID.
const ContextKeyRequestID = "request_id"

// RequestIDMiddleware generates a unique request ID for every request.
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}
		c.Set(ContextKeyRequestID, reqID)
		c.Header("X-Request-ID", reqID)
		c.Next()
	}
}
