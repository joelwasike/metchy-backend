package handler

import (
	"math/rand"
	"net/http"
	"time"

	"lusty/config"
	"lusty/internal/middleware"
	"lusty/internal/models"
	"lusty/internal/repository"
	"lusty/pkg/location"

	"github.com/gin-gonic/gin"
)

type LocationHandler struct {
	locRepo *repository.LocationRepository
	presenceRepo *repository.PresenceRepository
	companionRepo *repository.CompanionRepository
	cfg     *config.Config
	mapHub  interface{ UpdateLocation(companionID uint, lat, lng float64, isOnline bool) }
}

func NewLocationHandler(locRepo *repository.LocationRepository, presenceRepo *repository.PresenceRepository, companionRepo *repository.CompanionRepository, cfg *config.Config, mapHub interface{ UpdateLocation(uint, float64, float64, bool) }) *LocationHandler {
	return &LocationHandler{
		locRepo:       locRepo,
		presenceRepo:   presenceRepo,
		companionRepo: companionRepo,
		cfg:           cfg,
		mapHub:        mapHub,
	}
}

func (h *LocationHandler) UpdateLocation(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req struct {
		Latitude        float64 `json:"latitude" binding:"required"`
		Longitude       float64 `json:"longitude" binding:"required"`
		AccuracyMeters  float64 `json:"accuracy_meters"`
		IsLocationVisible *bool  `json:"is_location_visible"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	loc, _ := h.locRepo.GetByUserID(userID)
	if loc == nil {
		loc = &models.UserLocation{UserID: userID}
	}
	loc.Latitude = req.Latitude
	loc.Longitude = req.Longitude
	loc.AccuracyMeters = req.AccuracyMeters
	loc.LastUpdatedAt = time.Now()
	if req.IsLocationVisible != nil {
		loc.IsLocationVisible = *req.IsLocationVisible
	}
	if err := h.locRepo.Upsert(loc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	// If companion and online, push fuzzed location to map hub
	presence, _ := h.presenceRepo.GetByUserID(userID)
	isOnline := presence != nil && presence.IsOnline
	profile, err := h.companionRepo.GetByUserID(userID)
	if err == nil && profile != nil && isOnline && loc.IsLocationVisible && h.mapHub != nil {
		fuzz := h.cfg.Location.LocationFuzzMeters
		latFuzz := location.FuzzMeters(fuzz * (2*rand.Float64() - 1))
		lngFuzz := location.FuzzMeters(fuzz * (2*rand.Float64() - 1))
		h.mapHub.UpdateLocation(profile.ID, loc.Latitude+latFuzz, loc.Longitude+lngFuzz, true)
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *LocationHandler) GetMyLocation(c *gin.Context) {
	userID := middleware.GetUserID(c)
	loc, err := h.locRepo.GetByUserID(userID)
	if err != nil || loc == nil {
		c.JSON(http.StatusOK, gin.H{"latitude": nil, "longitude": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"latitude":            loc.Latitude,
		"longitude":           loc.Longitude,
		"accuracy_meters":     loc.AccuracyMeters,
		"is_location_visible": loc.IsLocationVisible,
		"last_updated_at":     loc.LastUpdatedAt,
	})
}
