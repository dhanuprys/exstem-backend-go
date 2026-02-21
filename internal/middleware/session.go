package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stemsi/exstem-backend/internal/response"
	"github.com/stemsi/exstem-backend/internal/service"
)

// CheckSingleDeviceSession validates the JWT's JTI against the active session in Redis.
// If the JTI doesn't match, the request is rejected (the session was reset by admin).
func CheckSingleDeviceSession(authService *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := GetClaims(c)
		if claims == nil {
			response.AbortFail(c, http.StatusUnauthorized, response.ErrTokenRequired)
			return
		}

		// Only enforce for student tokens.
		if claims.TokenType != service.TokenTypeStudent {
			c.Next()
			return
		}

		if err := authService.ValidateStudentSession(c.Request.Context(), claims.UserID, claims.ID); err != nil {
			response.AbortFail(c, http.StatusUnauthorized, response.ErrSessionInvalidated)
			return
		}

		c.Next()
	}
}
