package handler

import (
	"net/http"
	"strconv"

	"lusty/internal/middleware"
	"lusty/internal/repository"

	"github.com/gin-gonic/gin"
)

type FavoriteHandler struct {
	repo         *repository.FavoriteRepository
	companionRepo *repository.CompanionRepository
}

func NewFavoriteHandler(repo *repository.FavoriteRepository, companionRepo *repository.CompanionRepository) *FavoriteHandler {
	return &FavoriteHandler{repo: repo, companionRepo: companionRepo}
}

func (h *FavoriteHandler) Add(c *gin.Context) {
	clientID := middleware.GetUserID(c)
	companionID, _ := strconv.ParseUint(c.Param("companion_id"), 10, 64)
	ok, _ := h.repo.IsFavorite(clientID, uint(companionID))
	if ok {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}
	if err := h.repo.Add(clientID, uint(companionID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add favorite"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": "ok"})
}

func (h *FavoriteHandler) Remove(c *gin.Context) {
	clientID := middleware.GetUserID(c)
	companionID, _ := strconv.ParseUint(c.Param("companion_id"), 10, 64)
	if err := h.repo.Remove(clientID, uint(companionID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *FavoriteHandler) List(c *gin.Context) {
	clientID := middleware.GetUserID(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	list, err := h.repo.ListByClientID(clientID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list failed"})
		return
	}
	out := make([]gin.H, len(list))
	for i, f := range list {
		companion := gin.H{
			"id": f.CompanionID,
			"companion_id": f.CompanionID,
			"display_name": "",
			"main_profile_image_url": "",
			"city_or_area": "",
		}
		if prof, _ := h.companionRepo.GetByID(f.CompanionID); prof != nil {
			companion["display_name"] = prof.DisplayName
			companion["main_profile_image_url"] = prof.MainProfileImageURL
			companion["city_or_area"] = prof.CityOrArea
		}
		out[i] = gin.H{
			"id":            f.ID,
			"client_id":     f.ClientID,
			"companion_id":  f.CompanionID,
			"created_at":    f.CreatedAt,
			"companion":     companion,
		}
	}
	c.JSON(http.StatusOK, gin.H{"favorites": out})
}
