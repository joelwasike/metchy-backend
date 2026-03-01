package middleware

import (
	"net/http"

	"lusty/internal/domain"

	"github.com/gin-gonic/gin"
)

// AdminRequired checks that the authenticated user has the ADMIN role.
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role.(string) != domain.RoleAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			return
		}
		c.Next()
	}
}
