package middleware

import (
	"net/http"
	"time"

	"lusty/config"
	"lusty/internal/repository"

	"github.com/gin-gonic/gin"
)

const minAge = 18

// AdultOnly ensures the user is 18+ (DOB verified). Use after AuthRequired.
func AdultOnly(cfg *config.Config, userRepo *repository.UserRepository) gin.HandlerFunc {
	minAgeConfig := cfg.Location.MinAge
	if minAgeConfig < 18 {
		minAgeConfig = 18
	}
	return func(c *gin.Context) {
		userID := GetUserID(c)
		if userID == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		u, err := userRepo.GetByID(userID)
		if err != nil || u.DateOfBirth == nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "age verification required"})
			return
		}
		age := u.Age(time.Now())
		if age < minAgeConfig {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "must be 18 or older"})
			return
		}
		c.Next()
	}
}
