package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stemsi/exstem-backend/internal/response"
)

// RequirePermission checks that the admin JWT contains the required permission code.
func RequirePermission(permissionCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := GetClaims(c)
		if claims == nil {
			response.AbortFail(c, http.StatusUnauthorized, response.ErrTokenRequired)
			return
		}

		for _, p := range claims.Permissions {
			if p == permissionCode {
				c.Next()
				return
			}
		}

		response.AbortFail(c, http.StatusForbidden, response.ErrPermissionDenied)
	}
}

// RequireAnyPermission checks that the admin JWT contains at least one of the specified permissions.
func RequireAnyPermission(codes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := GetClaims(c)
		if claims == nil {
			response.AbortFail(c, http.StatusUnauthorized, response.ErrTokenRequired)
			return
		}

		for _, p := range claims.Permissions {
			for _, code := range codes {
				if p == code {
					c.Next()
					return
				}
			}
		}

		response.AbortFail(c, http.StatusForbidden, response.ErrPermissionDenied)
	}
}
