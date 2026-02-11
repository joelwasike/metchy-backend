package middleware

import (
	"net/http"
	"strings"

	"lusty/config"
	"lusty/internal/auth"

	"github.com/gin-gonic/gin"
)

// AuthRequired validates JWT and sets UserID, Email, Role in context.
func AuthRequired(cfg *config.JWTConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			return
		}
		claims, err := auth.ParseAccessToken(cfg, parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)
		c.Set("claims", claims)
		c.Next()
	}
}

// RequireRole checks that the authenticated user has one of the allowed roles.
func RequireRole(allowed ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		r := role.(string)
		for _, a := range allowed {
			if r == a {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	}
}

// GetUserID returns the authenticated user ID from context (must be used after AuthRequired).
func GetUserID(c *gin.Context) uint {
	v, _ := c.Get("user_id")
	if v == nil {
		return 0
	}
	return v.(uint)
}
