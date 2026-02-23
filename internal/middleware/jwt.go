package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/stemsi/exstem-backend/internal/response"
	"github.com/stemsi/exstem-backend/internal/service"
)

const (
	// ContextKeyClaims is the Gin context key for JWT claims.
	ContextKeyClaims = "claims"
)

// RequireStudentJWT validates a student JWT from the Authorization header.
func RequireStudentJWT(authService *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := extractAndValidateClaims(c, authService)
		if err != nil {
			response.AbortFail(c, http.StatusUnauthorized, response.ErrTokenInvalid)
			return
		}

		if claims.TokenType != service.TokenTypeStudent {
			response.AbortFail(c, http.StatusForbidden, response.ErrStudentAccessOnly)
			return
		}

		c.Set(ContextKeyClaims, claims)
		c.Next()
	}
}

// RequireAdminJWT validates an admin JWT from the Authorization header.
func RequireAdminJWT(authService *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := extractAndValidateClaims(c, authService)
		if err != nil {
			response.AbortFail(c, http.StatusUnauthorized, response.ErrTokenInvalid)
			return
		}

		if claims.TokenType != service.TokenTypeAdmin {
			response.AbortFail(c, http.StatusForbidden, response.ErrAdminAccessOnly)
			return
		}

		c.Set(ContextKeyClaims, claims)
		c.Next()
	}
}

// RequireStudentWSAuth validates a student JWT from the query param ?token=...
// Used for WebSocket upgrade requests.
func RequireStudentWSAuth(authService *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.Query("token")
		if tokenStr == "" {
			response.AbortFail(c, http.StatusUnauthorized, response.ErrTokenRequired)
			return
		}

		claims, err := authService.ValidateToken(tokenStr)
		if err != nil {
			response.AbortFail(c, http.StatusUnauthorized, response.ErrTokenInvalid)
			return
		}

		if claims.TokenType != service.TokenTypeStudent {
			response.AbortFail(c, http.StatusForbidden, response.ErrStudentAccessOnly)
			return
		}

		c.Set(ContextKeyClaims, claims)
		c.Next()
	}
}

// GetClaims retrieves the JWT claims from the Gin context.
func GetClaims(c *gin.Context) *service.Claims {
	val, exists := c.Get(ContextKeyClaims)
	if !exists {
		return nil
	}
	claims, ok := val.(*service.Claims)
	if !ok {
		return nil
	}
	return claims
}

func extractAndValidateClaims(c *gin.Context, authService *service.AuthService) (*service.Claims, error) {
	tokenStr := ""

	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			tokenStr = parts[1]
		}
	}

	// Fallback for EventSource (SSE) which cannot send headers
	if tokenStr == "" {
		tokenStr = c.Query("token")
	}

	if tokenStr == "" {
		return nil, fmt.Errorf("authorization header or token query required")
	}

	return authService.ValidateToken(tokenStr)
}
