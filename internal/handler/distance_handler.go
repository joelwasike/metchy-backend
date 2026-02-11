package handler

import (
	"net/http"
	"strconv"

	"lusty/internal/domain"
	"lusty/internal/middleware"
	"lusty/internal/repository"
	"lusty/pkg/location"

	"github.com/gin-gonic/gin"
)

// DistanceHandler returns distance between client and companion for accepted interactions (no map, just distance).
type DistanceHandler struct {
	interactionRepo *repository.InteractionRepository
	companionRepo   *repository.CompanionRepository
	locRepo         *repository.LocationRepository
	userRepo        *repository.UserRepository
}

func NewDistanceHandler(
	interactionRepo *repository.InteractionRepository,
	companionRepo *repository.CompanionRepository,
	locRepo *repository.LocationRepository,
	userRepo *repository.UserRepository,
) *DistanceHandler {
	return &DistanceHandler{
		interactionRepo: interactionRepo,
		companionRepo:   companionRepo,
		locRepo:         locRepo,
		userRepo:        userRepo,
	}
}

// GetDistance returns distance in km between the authenticated user and the companion for an accepted interaction.
// Client can poll this to see "the lady is coming" as distance decreases (no map, just the number).
func (h *DistanceHandler) GetDistance(c *gin.Context) {
	userID := middleware.GetUserID(c)
	interactionIDStr := c.Param("interaction_id")
	interactionID, _ := strconv.ParseUint(interactionIDStr, 10, 64)
	if interactionID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid interaction_id"})
		return
	}
	ir, err := h.interactionRepo.GetByID(uint(interactionID))
	if err != nil || ir == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if ir.Status != domain.RequestStatusAccepted {
		c.JSON(http.StatusForbidden, gin.H{"error": "interaction must be accepted"})
		return
	}
	if userID != ir.ClientID {
		c.JSON(http.StatusForbidden, gin.H{"error": "clients only"})
		return
	}
	clientLoc, err := h.locRepo.GetByUserID(ir.ClientID)
	if err != nil || clientLoc == nil {
		c.JSON(http.StatusOK, gin.H{
			"distance_km":     nil,
			"message":         "Update your location to see distance",
			"client_has_location": false,
			"companion_has_location": false,
		})
		return
	}
	companion, _ := h.companionRepo.GetByID(ir.CompanionID)
	if companion == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "companion not found"})
		return
	}
	companionLoc, err := h.locRepo.GetByUserID(companion.UserID)
	if err != nil || companionLoc == nil {
		c.JSON(http.StatusOK, gin.H{
			"distance_km":     nil,
			"message":         "Companion location not yet available",
			"client_has_location": true,
			"companion_has_location": false,
		})
		return
	}
	distKm := location.HaversineKm(
		clientLoc.Latitude, clientLoc.Longitude,
		companionLoc.Latitude, companionLoc.Longitude,
	)
	distRounded := float64(int(distKm*100+0.5)) / 100
	c.JSON(http.StatusOK, gin.H{
		"distance_km":           distRounded,
		"companion_location_at": companionLoc.LastUpdatedAt,
		"client_has_location":   true,
		"companion_has_location": true,
	})
}
