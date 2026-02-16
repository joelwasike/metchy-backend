package handler

import (
	"net/http"
	"time"

	"lusty/internal/middleware"
	"lusty/internal/models"
	"lusty/internal/repository"

	"github.com/gin-gonic/gin"
)

// Boost duration from type string
var boostDurations = map[string]time.Duration{
	"1h":   1 * time.Hour,
	"24h":  24 * time.Hour,
	"72h":  72 * time.Hour,
	"30d":  30 * 24 * time.Hour, // 1 month
}

type BoostHandler struct {
	companionRepo *repository.CompanionRepository
}

func NewBoostHandler(companionRepo *repository.CompanionRepository) *BoostHandler {
	return &BoostHandler{companionRepo: companionRepo}
}

// Activate creates a boost for the companion. In production, payment_reference would be verified first.
func (h *BoostHandler) Activate(c *gin.Context) {
	userID := middleware.GetUserID(c)
	profile, err := h.companionRepo.GetByUserID(userID)
	if err != nil || profile == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "companion profile required"})
		return
	}
	var req struct {
		BoostType        string `json:"boost_type" binding:"required"` // 1h, 24h, 72h, 30d
		PaymentReference string `json:"payment_reference"`              // optional; verify via payment provider
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	dur, ok := boostDurations[req.BoostType]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid boost_type (use 1h, 24h, 72h, 30d)"})
		return
	}
	now := time.Now()
	b := &models.CompanionBoost{
		CompanionID: profile.ID,
		BoostType:   req.BoostType,
		StartAt:     now,
		EndAt:       now.Add(dur),
		IsActive:    true,
	}
	if err := h.companionRepo.CreateBoost(b); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "activate failed"})
		return
	}
	_ = req.PaymentReference
	c.JSON(http.StatusCreated, b)
}
