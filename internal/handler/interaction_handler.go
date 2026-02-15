package handler

import (
	"net/http"
	"strconv"
	"time"

	"lusty/internal/domain"
	"lusty/internal/middleware"
	"lusty/internal/models"
	"lusty/internal/repository"
	"lusty/internal/service"

	"github.com/gin-gonic/gin"
)

type InteractionHandler struct {
	interactionRepo *repository.InteractionRepository
	companionRepo   *repository.CompanionRepository
	paymentRepo     *repository.PaymentRepository
	walletRepo      *repository.WalletRepository
	userRepo        *repository.UserRepository
	notifSvc        *service.NotificationService
}

func NewInteractionHandler(
	interactionRepo *repository.InteractionRepository,
	companionRepo *repository.CompanionRepository,
	paymentRepo *repository.PaymentRepository,
	walletRepo *repository.WalletRepository,
	userRepo *repository.UserRepository,
	notifSvc *service.NotificationService,
) *InteractionHandler {
	return &InteractionHandler{
		interactionRepo: interactionRepo,
		companionRepo:   companionRepo,
		paymentRepo:     paymentRepo,
		walletRepo:      walletRepo,
		userRepo:        userRepo,
		notifSvc:        notifSvc,
	}
}

func (h *InteractionHandler) Create(c *gin.Context) {
	clientID := middleware.GetUserID(c)
	var req struct {
		CompanionID     uint   `json:"companion_id" binding:"required"`
		InteractionType string `json:"interaction_type" binding:"required,oneof=CHAT VIDEO BOOKING"`
		PaymentID       *uint  `json:"payment_id"`
		PaymentRef      string `json:"payment_reference"`
		DurationMinutes int    `json:"duration_minutes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var paymentID *uint
	if req.PaymentID != nil {
		paymentID = req.PaymentID
	} else if req.PaymentRef != "" {
		p, err := h.paymentRepo.GetByProviderRef(req.PaymentRef)
		if err != nil || p == nil || p.Status != "COMPLETED" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "valid payment required"})
			return
		}
		paymentID = &p.ID
	}
	if paymentID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payment_id or payment_reference required"})
		return
	}
	expiresAt := time.Now().Add(30 * time.Minute)
	ir := &models.InteractionRequest{
		ClientID:         clientID,
		CompanionID:      req.CompanionID,
		InteractionType:  req.InteractionType,
		PaymentID:        paymentID,
		Status:           domain.RequestStatusPending,
		DurationMinutes: req.DurationMinutes,
		ExpiresAt:        &expiresAt,
	}
	if err := h.interactionRepo.Create(ir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create failed"})
		return
	}
	companion, _ := h.companionRepo.GetByID(req.CompanionID)
	if companion != nil {
		client, _ := h.userRepo.GetByID(clientID)
		clientName := ""
		if client != nil {
			if client.Username != "" {
				clientName = client.Username
			} else {
				clientName = client.Email
			}
		}
		_ = h.notifSvc.NotifyNewRequest(companion.UserID, ir.ID, clientName)
	}
	c.JSON(http.StatusCreated, ir)
}

func (h *InteractionHandler) ListMine(c *gin.Context) {
	userID := middleware.GetUserID(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	role, _ := c.Get("role")
	roleStr, _ := role.(string)
	if roleStr == domain.RoleCompanion {
		profile, _ := h.companionRepo.GetByUserID(userID)
		if profile == nil {
			c.JSON(http.StatusOK, gin.H{"requests": []interface{}{}})
			return
		}
		list, err := h.interactionRepo.ListByCompanionID(profile.ID, limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "list failed"})
			return
		}
		out := make([]gin.H, 0, len(list))
		for _, ir := range list {
			clientDisplay := ""
			clientAvatarURL := ""
			if c, _ := h.userRepo.GetByID(ir.ClientID); c != nil {
				if c.Username != "" {
					clientDisplay = c.Username
				} else {
					clientDisplay = c.Email
				}
				clientAvatarURL = c.AvatarURL
			}
			paymentStatus := ""
			if ir.Payment != nil {
				paymentStatus = ir.Payment.Status
			}
			entry := gin.H{
				"id":                 ir.ID,
				"client_id":          ir.ClientID,
				"companion_id":       ir.CompanionID,
				"interaction_type":   ir.InteractionType,
				"payment_id":         ir.PaymentID,
				"status":             ir.Status,
				"payment_status":     paymentStatus,
				"duration_minutes":   ir.DurationMinutes,
				"created_at":         ir.CreatedAt,
				"client":             gin.H{"username": clientDisplay, "email": clientDisplay, "avatar_url": clientAvatarURL},
				"payment":            ir.Payment,
			}
			if ir.Status == domain.RequestStatusAccepted {
				if session, _ := h.interactionRepo.GetChatSessionByInteractionID(ir.ID); session != nil {
					entry["session_ends_at"] = session.EndsAt
					entry["session_ended"] = session.EndedAt != nil
				}
			}
			out = append(out, entry)
		}
		c.JSON(http.StatusOK, gin.H{"requests": out})
		return
	}
	list, err := h.interactionRepo.ListByClientID(userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list failed"})
		return
	}
	// Include companion display_name, main_profile_image_url and session_ends_at for chat list
	out := make([]gin.H, 0, len(list))
	for _, ir := range list {
		companionName := ""
		mainImageURL := ""
		if ir.Companion.ID != 0 {
			companionName = ir.Companion.DisplayName
			mainImageURL = ir.Companion.MainProfileImageURL
		}
		entry := gin.H{
			"id":                 ir.ID,
			"client_id":          ir.ClientID,
			"companion_id":       ir.CompanionID,
			"interaction_type":   ir.InteractionType,
			"payment_id":         ir.PaymentID,
			"status":             ir.Status,
			"service_completed":  ir.ServiceCompletedAt != nil,
			"duration_minutes":   ir.DurationMinutes,
			"created_at":         ir.CreatedAt,
			"companion":          gin.H{"display_name": companionName, "main_profile_image_url": mainImageURL},
		}
		if ir.Status == domain.RequestStatusAccepted {
			if session, _ := h.interactionRepo.GetChatSessionByInteractionID(ir.ID); session != nil {
				entry["session_ends_at"] = session.EndsAt
				entry["session_ended"] = session.EndedAt != nil
			}
		}
		out = append(out, entry)
	}
	c.JSON(http.StatusOK, gin.H{"requests": out})
}

func (h *InteractionHandler) Accept(c *gin.Context) {
	userID := middleware.GetUserID(c)
	profile, err := h.companionRepo.GetByUserID(userID)
	if err != nil || profile == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "companion only"})
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	ir, err := h.interactionRepo.GetByID(uint(id))
	if err != nil || ir == nil || ir.CompanionID != profile.ID {
		c.JSON(http.StatusNotFound, gin.H{"error": "request not found"})
		return
	}
	if ir.Status != domain.RequestStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request not pending"})
		return
	}
	// Only allow accept when payment is completed
	if ir.PaymentID == nil || ir.Payment == nil || ir.Payment.Status != "COMPLETED" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payment not completed yet"})
		return
	}
	now := time.Now()
	ir.Status = domain.RequestStatusAccepted
	ir.AcceptedAt = &now
	if err := h.interactionRepo.Update(ir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "accept failed"})
		return
	}
	endsAt := now.Add(time.Duration(ir.DurationMinutes) * time.Minute)
	if ir.DurationMinutes <= 0 {
		endsAt = now.Add(24 * time.Hour)
	}
	session := &models.ChatSession{
		InteractionID: ir.ID,
		StartedAt:     now,
		EndsAt:        endsAt,
	}
	_ = h.interactionRepo.CreateChatSession(session)
	// Credit companion's wallet (withdrawable after client confirms service done)
	_ = h.walletRepo.Credit(profile.UserID, ir.Payment.AmountCents)
	_ = h.notifSvc.NotifyAccepted(ir.ClientID, profile.DisplayName, ir.ID)
	c.JSON(http.StatusOK, ir)
}

func (h *InteractionHandler) Reject(c *gin.Context) {
	userID := middleware.GetUserID(c)
	profile, err := h.companionRepo.GetByUserID(userID)
	if err != nil || profile == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "companion only"})
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	ir, err := h.interactionRepo.GetByID(uint(id))
	if err != nil || ir == nil || ir.CompanionID != profile.ID {
		c.JSON(http.StatusNotFound, gin.H{"error": "request not found"})
		return
	}
	if ir.Status != domain.RequestStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request not pending"})
		return
	}
	// Refund to client wallet if payment was completed
	if ir.PaymentID != nil && ir.Payment != nil && ir.Payment.Status == "COMPLETED" {
		_ = h.walletRepo.Credit(ir.ClientID, ir.Payment.AmountCents)
	}
	now := time.Now()
	ir.Status = domain.RequestStatusRejected
	ir.RejectedAt = &now
	if err := h.interactionRepo.Update(ir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "reject failed"})
		return
	}
	client, _ := h.userRepo.GetByID(ir.ClientID)
	if client != nil {
		_ = h.notifSvc.NotifyRejected(ir.ClientID, profile.DisplayName)
	}
	c.JSON(http.StatusOK, ir)
}

// ServiceDone is called by the client when they confirm the service is complete.
// Marks interaction as done, ends chat session, deletes messages, credits companion's withdrawable balance.
func (h *InteractionHandler) ServiceDone(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role, _ := c.Get("role")
	if roleStr, _ := role.(string); roleStr != domain.RoleClient {
		c.JSON(http.StatusForbidden, gin.H{"error": "client only"})
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	ir, err := h.interactionRepo.GetByID(uint(id))
	if err != nil || ir == nil || ir.ClientID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "interaction not found"})
		return
	}
	if ir.Status != domain.RequestStatusAccepted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "interaction not accepted"})
		return
	}
	if ir.ServiceCompletedAt != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service already marked done"})
		return
	}
	now := time.Now()
	ir.ServiceCompletedAt = &now
	if err := h.interactionRepo.Update(ir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	session, _ := h.interactionRepo.GetChatSessionByInteractionID(ir.ID)
	if session != nil {
		session.EndedAt = &now
		_ = h.interactionRepo.UpdateChatSession(session)
		_ = h.interactionRepo.DeleteMessagesBySessionID(session.ID)
	}
	// Credit companion's withdrawable balance so they can withdraw
	if ir.PaymentID != nil && ir.Payment != nil && ir.Payment.Status == "COMPLETED" {
		comp, _ := h.companionRepo.GetByID(ir.CompanionID)
		if comp != nil {
			_ = h.walletRepo.CreditWithdrawable(comp.UserID, ir.Payment.AmountCents)
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Service confirmed. Companion can now withdraw."})
}
