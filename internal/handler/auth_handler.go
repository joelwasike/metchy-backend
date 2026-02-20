package handler

import (
	"log"
	"net/http"
	"strings"
	"time"

	"lusty/internal/domain"
	"lusty/internal/middleware"
	"lusty/internal/models"
	"lusty/internal/repository"
	"lusty/internal/service"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	svc           *service.AuthService
	presenceRepo  *repository.PresenceRepository
	auditRepo     *repository.AuditLogRepository
	companionRepo *repository.CompanionRepository
	referralRepo  *repository.ReferralRepository
}

func NewAuthHandler(svc *service.AuthService, presenceRepo *repository.PresenceRepository, auditRepo *repository.AuditLogRepository, companionRepo *repository.CompanionRepository, referralRepo *repository.ReferralRepository) *AuthHandler {
	return &AuthHandler{svc: svc, presenceRepo: presenceRepo, auditRepo: auditRepo, companionRepo: companionRepo, referralRepo: referralRepo}
}

type RegisterRequest struct {
	Email        string `json:"email" binding:"required,email"`
	Username     string `json:"username" binding:"required,min=3,max=64"`
	Password     string `json:"password" binding:"required,min=8"`
	Role         string `json:"role" binding:"required,oneof=CLIENT COMPANION"`
	DateOfBirth  string `json:"date_of_birth" binding:"required"` // ISO date
	ReferralCode string `json:"referral_code"`                    // optional: referrer's code
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	dob, err := time.Parse("2006-01-02", req.DateOfBirth)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date_of_birth format (use YYYY-MM-DD)"})
		return
	}
	u, access, refresh, err := h.svc.Register(req.Email, req.Username, req.Password, req.Role, dob)
	if err != nil {
		switch err {
		case service.ErrEmailExists:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case service.ErrUsernameExists:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case service.ErrAgeRequired:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			log.Printf("[auth] register failed: role=%s email=%s err=%v", req.Role, req.Email, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "registration failed"})
		}
		return
	}
	h.setPresenceOnline(u.ID)
	h.auditLog(u.ID, "register", c)

	// Process referral code if provided
	if req.ReferralCode != "" && h.referralRepo != nil {
		rc, err := h.referralRepo.GetByCode(req.ReferralCode)
		if err == nil && rc != nil && rc.UserID != u.ID {
			_ = h.referralRepo.CreateReferral(&models.Referral{
				ReferrerID:     rc.UserID,
				ReferredUserID: u.ID,
			})
		}
	}

	// Auto-create CompanionProfile for COMPANION (needs onboarding to complete)
	if u.Role == domain.RoleCompanion && h.companionRepo != nil {
		displayName := u.Username
		if displayName == "" {
			displayName, _, _ = strings.Cut(u.Email, "@")
		}
		if displayName == "" {
			displayName = "Companion"
		}
		_ = h.companionRepo.Create(&models.CompanionProfile{
			UserID:            u.ID,
			DisplayName:       displayName,
			AppearInSearch:    false, // until onboarding complete
			AcceptNewRequests: true,
		})
	}

	c.JSON(http.StatusCreated, gin.H{
		"user":          u,
		"access_token":  access,
		"refresh_token": refresh,
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	u, access, refresh, err := h.svc.Login(req.Email, req.Password)
	if err != nil {
		if err == service.ErrInvalidCreds {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		return
	}
	h.setPresenceOnline(u.ID)
	h.auditLog(u.ID, "login", c)
	c.JSON(http.StatusOK, gin.H{
		"user":          u,
		"access_token":  access,
		"refresh_token": refresh,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}
	h.setPresenceOffline(userID)
	h.auditLog(userID, "logout", c)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *AuthHandler) setPresenceOnline(userID uint) {
	if h.presenceRepo == nil {
		return
	}
	presence, _ := h.presenceRepo.GetByUserID(userID)
	if presence == nil {
		presence = &models.UserPresence{UserID: userID}
	}
	presence.Status = domain.PresenceOnline
	presence.IsOnline = true
	presence.LastSeenAt = time.Now()
	_ = h.presenceRepo.Upsert(presence)
}

func (h *AuthHandler) setPresenceOffline(userID uint) {
	if h.presenceRepo == nil {
		return
	}
	presence, _ := h.presenceRepo.GetByUserID(userID)
	if presence == nil {
		presence = &models.UserPresence{UserID: userID}
	}
	presence.Status = domain.PresenceOffline
	presence.IsOnline = false
	presence.LastSeenAt = time.Now()
	_ = h.presenceRepo.Upsert(presence)
}

func (h *AuthHandler) auditLog(userID uint, action string, c *gin.Context) {
	if h.auditRepo == nil {
		return
	}
	_ = h.auditRepo.Create(&models.AuditLog{
		UserID:   &userID,
		Action:   action,
		Resource: "auth",
		IP:       c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	})
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.ChangePassword(userID, req.CurrentPassword, req.NewPassword); err != nil {
		if err == service.ErrInvalidCreds {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Current password is incorrect"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	access, refresh, err := h.svc.RefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token":  access,
		"refresh_token": refresh,
	})
}
