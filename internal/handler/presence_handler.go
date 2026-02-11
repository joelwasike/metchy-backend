package handler

import (
	"net/http"
	"time"

	"lusty/internal/domain"
	"lusty/internal/middleware"
	"lusty/internal/models"
	"lusty/internal/repository"
	"lusty/internal/service"

	"github.com/gin-gonic/gin"
)

type PresenceHandler struct {
	repo          *repository.PresenceRepository
	companionRepo *repository.CompanionRepository
	favRepo       *repository.FavoriteRepository
	notifSvc      *service.NotificationService
}

func NewPresenceHandler(repo *repository.PresenceRepository, companionRepo *repository.CompanionRepository, favRepo *repository.FavoriteRepository, notifSvc *service.NotificationService) *PresenceHandler {
	return &PresenceHandler{repo: repo, companionRepo: companionRepo, favRepo: favRepo, notifSvc: notifSvc}
}

func (h *PresenceHandler) SetPresence(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req struct {
		Status string `json:"status" binding:"required,oneof=ONLINE OFFLINE BUSY IN_SESSION"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	presence, _ := h.repo.GetByUserID(userID)
	if presence == nil {
		presence = &models.UserPresence{UserID: userID}
	}
	wasOnline := presence.IsOnline
	presence.Status = req.Status
	presence.IsOnline = req.Status == domain.PresenceOnline
	presence.LastSeenAt = time.Now()
	if err := h.repo.Upsert(presence); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	// Notify clients who favorited this companion when they go online
	if h.notifSvc != nil && h.companionRepo != nil && h.favRepo != nil && presence.IsOnline && !wasOnline {
		profile, _ := h.companionRepo.GetByUserID(userID)
		if profile != nil {
			clientIDs, _ := h.favRepo.ListClientIDsByCompanionID(profile.ID)
			for _, cid := range clientIDs {
				_ = h.notifSvc.NotifyFavoriteOnline(cid, profile.DisplayName, profile.ID)
			}
		}
	}
	c.JSON(http.StatusOK, presence)
}

func (h *PresenceHandler) GetMyPresence(c *gin.Context) {
	userID := middleware.GetUserID(c)
	presence, err := h.repo.GetByUserID(userID)
	if err != nil || presence == nil {
		c.JSON(http.StatusOK, gin.H{"status": "OFFLINE", "is_online": false})
		return
	}
	c.JSON(http.StatusOK, presence)
}
