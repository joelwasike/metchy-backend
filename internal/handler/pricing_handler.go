package handler

import (
	"net/http"
	"strconv"

	"lusty/internal/middleware"
	"lusty/internal/models"
	"lusty/internal/repository"

	"github.com/gin-gonic/gin"
)

type PricingHandler struct {
	companionRepo *repository.CompanionRepository
}

func NewPricingHandler(companionRepo *repository.CompanionRepository) *PricingHandler {
	return &PricingHandler{companionRepo: companionRepo}
}

func (h *PricingHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	profile, err := h.companionRepo.GetByUserID(userID)
	if err != nil || profile == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "companion profile required"})
		return
	}
	// Pricing loaded with profile
	c.JSON(http.StatusOK, gin.H{"pricing": profile.Pricing})
}

func (h *PricingHandler) Create(c *gin.Context) {
	userID := middleware.GetUserID(c)
	profile, err := h.companionRepo.GetByUserID(userID)
	if err != nil || profile == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "companion profile required"})
		return
	}
	var req struct {
		Type        string `json:"type" binding:"required"`
		Unit        string `json:"unit"` // per_service, per_hour, per_night (for service pricing)
		CustomName  string `json:"custom_name"`
		AmountCents int64  `json:"amount_cents" binding:"required,min=0"`
		Currency    string `json:"currency"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	allowedTypes := map[string]bool{
		"CHAT_ACCESS": true, "VIDEO_PER_5MIN": true, "BOOKING_FEE": true,
		"SEX": true, "HOOKUP": true, "MASSAGE": true, "NUDE_VIDEO_CALL": true,
		"THREESOME": true, "BJ_HANDJOB": true, "SHAVING_WAXING": true, "LAPDANCE": true, "OTHER": true,
		"CUSTOM": true,
	}
	if !allowedTypes[req.Type] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pricing type"})
		return
	}
	if req.Type == "CUSTOM" && req.CustomName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "custom_name required for CUSTOM type"})
		return
	}
	if req.Currency == "" {
		req.Currency = "USD"
	}
	// CUSTOM type: always create new (allow multiple custom services)
	if req.Type == "CUSTOM" {
		p := &models.CompanionPricing{
			CompanionID: profile.ID,
			Type:        req.Type,
			Unit:        req.Unit,
			CustomName:  req.CustomName,
			AmountCents: req.AmountCents,
			Currency:    req.Currency,
			IsActive:    true,
		}
		if err := h.companionRepo.UpsertPricing(p); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "create failed"})
			return
		}
		c.JSON(http.StatusCreated, p)
		return
	}
	// Upsert: update existing pricing for this companion+type instead of creating duplicate
	existing, _ := h.companionRepo.GetPricingByCompanionAndType(profile.ID, req.Type)
	if existing != nil {
		existing.AmountCents = req.AmountCents
		if req.Unit != "" {
			existing.Unit = req.Unit
		}
		if req.Currency != "" {
			existing.Currency = req.Currency
		}
		existing.IsActive = true
		if err := h.companionRepo.UpsertPricing(existing); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
			return
		}
		c.JSON(http.StatusOK, existing)
		return
	}
	p := &models.CompanionPricing{
		CompanionID: profile.ID,
		Type:        req.Type,
		Unit:        req.Unit,
		AmountCents: req.AmountCents,
		Currency:    req.Currency,
		IsActive:    true,
	}
	if err := h.companionRepo.UpsertPricing(p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create failed"})
		return
	}
	c.JSON(http.StatusCreated, p)
}

func (h *PricingHandler) Update(c *gin.Context) {
	userID := middleware.GetUserID(c)
	profile, err := h.companionRepo.GetByUserID(userID)
	if err != nil || profile == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "companion profile required"})
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		AmountCents *int64  `json:"amount_cents"`
		Unit        *string `json:"unit"`
		CustomName  *string `json:"custom_name"`
		Currency    string  `json:"currency"`
		IsActive    *bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := h.companionRepo.GetPricingByID(uint(id), profile.ID)
	if err != nil || p == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "pricing not found"})
		return
	}
	if req.AmountCents != nil {
		p.AmountCents = *req.AmountCents
	}
	if req.Unit != nil {
		p.Unit = *req.Unit
	}
	if req.CustomName != nil {
		p.CustomName = *req.CustomName
	}
	if req.Currency != "" {
		p.Currency = req.Currency
	}
	if req.IsActive != nil {
		p.IsActive = *req.IsActive
	}
	if err := h.companionRepo.UpsertPricing(p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *PricingHandler) Delete(c *gin.Context) {
	userID := middleware.GetUserID(c)
	profile, err := h.companionRepo.GetByUserID(userID)
	if err != nil || profile == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "companion profile required"})
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.companionRepo.DeletePricing(uint(id), profile.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
