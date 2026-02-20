package handler

import (
	"net/http"
	"strconv"

	"lusty/internal/middleware"
	"lusty/internal/repository"

	"github.com/gin-gonic/gin"
)

type ReferralHandler struct {
	referralRepo *repository.ReferralRepository
}

func NewReferralHandler(referralRepo *repository.ReferralRepository) *ReferralHandler {
	return &ReferralHandler{referralRepo: referralRepo}
}

// GetMyReferralCode returns the authenticated user's referral code, creating one if it doesn't exist yet.
// GET /me/referral-code
func (h *ReferralHandler) GetMyReferralCode(c *gin.Context) {
	userID := middleware.GetUserID(c)
	rc, err := h.referralRepo.GetOrCreateCode(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get referral code"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":       rc.Code,
		"is_active":  rc.IsActive,
		"created_at": rc.CreatedAt,
	})
}

// GetMyReferrals returns the list of users the authenticated user has referred,
// along with how many qualifying commissions have been earned per referral (max 2).
// GET /me/referrals
func (h *ReferralHandler) GetMyReferrals(c *gin.Context) {
	userID := middleware.GetUserID(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	referrals, err := h.referralRepo.ListByReferrerID(userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list referrals"})
		return
	}

	out := make([]gin.H, 0, len(referrals))
	for _, ref := range referrals {
		username := ref.ReferredUser.Username
		if username == "" {
			username = ref.ReferredUser.Email
		}
		commissionsRemaining := 2 - ref.CompletedCount
		if commissionsRemaining < 0 {
			commissionsRemaining = 0
		}
		out = append(out, gin.H{
			"referred_user": gin.H{
				"username": username,
				"role":     ref.ReferredUser.Role,
			},
			"completed_count":       ref.CompletedCount,
			"commissions_remaining": commissionsRemaining,
			"created_at":            ref.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"referrals": out, "total": len(out)})
}
