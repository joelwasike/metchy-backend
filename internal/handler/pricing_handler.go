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
		Type        string `json:"type" binding:"required,oneof=CHAT_ACCESS VIDEO_PER_5MIN BOOKING_FEE"`
		AmountCents int64  `json:"amount_cents" binding:"required,min=0"`
		Currency    string `json:"currency"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Currency == "" {
		req.Currency = "USD"
	}
	p := &models.CompanionPricing{
		CompanionID: profile.ID,
		Type:        req.Type,
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
		AmountCents *int64 `json:"amount_cents"`
		Currency    string `json:"currency"`
		IsActive    *bool  `json:"is_active"`
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
