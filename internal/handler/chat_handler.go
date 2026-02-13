package handler

import (
	"net/http"
	"strconv"
	"time"

	"lusty/internal/domain"
	"lusty/internal/middleware"
	"lusty/internal/repository"

	"github.com/gin-gonic/gin"
)

type ChatHandler struct {
	interactionRepo *repository.InteractionRepository
	companionRepo   *repository.CompanionRepository
}

func NewChatHandler(interactionRepo *repository.InteractionRepository, companionRepo *repository.CompanionRepository) *ChatHandler {
	return &ChatHandler{interactionRepo: interactionRepo, companionRepo: companionRepo}
}

// GetMessages returns paginated messages for an accepted interaction (client or companion).
func (h *ChatHandler) GetMessages(c *gin.Context) {
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
		c.JSON(http.StatusForbidden, gin.H{"error": "interaction not accepted"})
		return
	}
	if userID != ir.ClientID && userID != ir.Companion.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	session, err := h.interactionRepo.GetChatSessionByInteractionID(uint(interactionID))
	if err != nil || session == nil {
		c.JSON(http.StatusOK, gin.H{"messages": []interface{}{}, "service_completed": ir.ServiceCompletedAt != nil, "session_ended": true})
		return
	}
	if session.EndedAt != nil {
		c.JSON(http.StatusOK, gin.H{"messages": []interface{}{}, "service_completed": true, "session_ended": true})
		return
	}
	if session.EndsAt.Before(time.Now()) {
		c.JSON(http.StatusForbidden, gin.H{"error": "chat access expired"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	list, err := h.interactionRepo.GetMessagesBySessionID(session.ID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list failed"})
		return
	}
	resp := gin.H{"messages": list, "service_completed": ir.ServiceCompletedAt != nil, "session_ended": session.EndedAt != nil}
	c.JSON(http.StatusOK, resp)
}
