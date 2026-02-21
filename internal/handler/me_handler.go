package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"lusty/internal/domain"
	"lusty/internal/middleware"
	"lusty/internal/repository"
	"lusty/internal/service"

	"github.com/gin-gonic/gin"
)

type MeHandler struct {
	userRepo        *repository.UserRepository
	companionRepo   *repository.CompanionRepository
	locRepo         *repository.LocationRepository
	favRepo         *repository.FavoriteRepository
	paymentRepo     *repository.PaymentRepository
	interactionRepo *repository.InteractionRepository
	walletRepo      *repository.WalletRepository
	notifSvc        *service.NotificationService
}

func NewMeHandler(
	userRepo *repository.UserRepository,
	companionRepo *repository.CompanionRepository,
	locRepo *repository.LocationRepository,
	favRepo *repository.FavoriteRepository,
	paymentRepo *repository.PaymentRepository,
	interactionRepo *repository.InteractionRepository,
	walletRepo *repository.WalletRepository,
	notifSvc *service.NotificationService,
) *MeHandler {
	return &MeHandler{
		userRepo:        userRepo,
		companionRepo:   companionRepo,
		locRepo:         locRepo,
		favRepo:         favRepo,
		paymentRepo:     paymentRepo,
		interactionRepo: interactionRepo,
		walletRepo:      walletRepo,
		notifSvc:        notifSvc,
	}
}

// RegisterFCMToken saves the FCM token for push notifications.
func (h *MeHandler) RegisterFCMToken(c *gin.Context) {
	userID := middleware.GetUserID(c)
	u, err := h.userRepo.GetByID(userID)
	if err != nil || u == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token required"})
		return
	}
	u.FCMToken = req.Token
	if err := h.userRepo.Update(u); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetProfile returns the current user with profile completeness for redirect logic.
func (h *MeHandler) GetProfile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role, _ := c.Get("role")
	roleStr, _ := role.(string)

	u, err := h.userRepo.GetByID(userID)
	if err != nil || u == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	resp := gin.H{
		"id":                u.ID,
		"email":             u.Email,
		"role":              u.Role,
		"date_of_birth":     u.DateOfBirth,
		"avatar_url":        u.AvatarURL,
		"search_radius_km":   u.SearchRadiusKm,
		"kyc":               u.KYC,
		"profile_complete":  true,
		"needs_onboarding":   false,
	}
	if u.SearchRadiusKm <= 0 {
		u.SearchRadiusKm = 10
	}

	if roleStr == domain.RoleCompanion {
		profile, err := h.companionRepo.GetByUserID(userID)
		if err != nil || profile == nil {
			resp["profile_complete"] = false
			resp["needs_onboarding"] = true
			resp["companion_profile"] = nil
		} else {
			resp["companion_profile"] = profile
			resp["profile_complete"] = profile.OnboardingCompletedAt != nil
			resp["needs_onboarding"] = profile.OnboardingCompletedAt == nil
		}
	} else {
		loc, _ := h.locRepo.GetByUserID(userID)
		resp["has_location"] = loc != nil
		if loc == nil {
			resp["profile_complete"] = false
		}
	}

	c.JSON(http.StatusOK, resp)
}

// UpdateSettings updates client search radius or companion toggles.
func (h *MeHandler) UpdateSettings(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role, _ := c.Get("role")
	roleStr, _ := role.(string)

	u, err := h.userRepo.GetByID(userID)
	if err != nil || u == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	var req struct {
		SearchRadiusKm    *float64 `json:"search_radius_km"`
		AppearInSearch    *bool    `json:"appear_in_search"`
		AcceptNewRequests *bool    `json:"accept_new_requests"`
		IsLocationVisible *bool    `json:"is_location_visible"`
		Available         *bool    `json:"available"` // companion: manual "I'm available" toggle
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if roleStr == domain.RoleClient {
		if req.SearchRadiusKm != nil {
			r := *req.SearchRadiusKm
			if r < 1 {
				r = 1
			}
			if r > 50 {
				r = 50
			}
			u.SearchRadiusKm = r
		}
		if req.IsLocationVisible != nil {
			loc, _ := h.locRepo.GetByUserID(userID)
			if loc != nil {
				loc.IsLocationVisible = *req.IsLocationVisible
				_ = h.locRepo.Upsert(loc)
			}
		}
		if err := h.userRepo.Update(u); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
			return
		}
	}

	if roleStr == domain.RoleCompanion {
		profile, err := h.companionRepo.GetByUserID(userID)
		if err != nil || profile == nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "companion profile required"})
			return
		}
		if req.AppearInSearch != nil {
			profile.AppearInSearch = *req.AppearInSearch
		}
		if req.AcceptNewRequests != nil {
			profile.AcceptNewRequests = *req.AcceptNewRequests
		}
		if req.Available != nil {
			profile.IsAvailable = *req.Available
		}
		if req.IsLocationVisible != nil {
			loc, _ := h.locRepo.GetByUserID(userID)
			if loc != nil {
				loc.IsLocationVisible = *req.IsLocationVisible
				_ = h.locRepo.Upsert(loc)
			}
		}
		if err := h.companionRepo.Update(profile); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetDashboard returns companion dashboard stats (earnings, boost, favorites, sessions).
func (h *MeHandler) GetDashboard(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role, _ := c.Get("role")
	if roleStr, _ := role.(string); roleStr != domain.RoleCompanion {
		c.JSON(http.StatusForbidden, gin.H{"error": "companion only"})
		return
	}

	profile, err := h.companionRepo.GetByUserID(userID)
	if err != nil || profile == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "companion profile required"})
		return
	}

	// Wallet: balance (from payments) and withdrawable (after client confirms service done)
	wallet, _ := h.walletRepo.GetOrCreate(userID)
	balanceCents := int64(0)
	withdrawableCents := int64(0)
	if wallet != nil {
		balanceCents = wallet.BalanceCents
		withdrawableCents = wallet.WithdrawableCents
	}

	// Boost status
	boost, _ := h.companionRepo.GetActiveBoost(profile.ID)
	boostEndsAt := (*time.Time)(nil)
	if boost != nil {
		boostEndsAt = &boost.EndAt
	}

	// Favorites count
	favCount, _ := h.favRepo.CountByCompanionID(profile.ID)

	// Active sessions (accepted, session not ended)
	activeSessions, _ := h.interactionRepo.CountActiveSessionsByCompanionID(profile.ID)

	// Pending requests count (for badge)
	pendingRequests, _ := h.interactionRepo.CountPendingByCompanionID(profile.ID)

	c.JSON(http.StatusOK, gin.H{
		"earnings_cents":        balanceCents,
		"withdrawable_cents":    withdrawableCents,
		"is_boosted":            boost != nil,
		"boost_ends_at":         boostEndsAt,
		"favorites_count":      favCount,
		"active_sessions":      activeSessions,
		"pending_requests":     pendingRequests,
		"is_available":         profile.IsAvailable,
	})
}

// GetActiveSessions returns active chat sessions for the companion (service type, duration, client name).
func (h *MeHandler) GetActiveSessions(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role, _ := c.Get("role")
	if roleStr, _ := role.(string); roleStr != domain.RoleCompanion {
		c.JSON(http.StatusForbidden, gin.H{"error": "companion only"})
		return
	}
	profile, err := h.companionRepo.GetByUserID(userID)
	if err != nil || profile == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "companion profile required"})
		return
	}
	list, err := h.interactionRepo.ListActiveSessionsByCompanionID(profile.ID, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load sessions"})
		return
	}
	out := make([]gin.H, 0, len(list))
	for _, row := range list {
		out = append(out, gin.H{
			"interaction_id":    row.InteractionID,
			"client_name":       row.ClientName,
			"service_type":      row.ServiceType,
			"duration_minutes":  row.DurationMinutes,
			"started_at":        row.StartedAt.Format(time.RFC3339),
			"ends_at":           row.EndsAt.Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, gin.H{"sessions": out})
}

// GetFans returns clients who favorited this companion (names only).
func (h *MeHandler) GetFans(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role, _ := c.Get("role")
	if roleStr, _ := role.(string); roleStr != domain.RoleCompanion {
		c.JSON(http.StatusForbidden, gin.H{"error": "companion only"})
		return
	}
	profile, err := h.companionRepo.GetByUserID(userID)
	if err != nil || profile == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "companion profile required"})
		return
	}
	limit, offset := 50, 0
	list, err := h.favRepo.ListFansByCompanionID(profile.ID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load fans"})
		return
	}
	out := make([]gin.H, 0, len(list))
	for _, e := range list {
		out = append(out, gin.H{"client_id": e.ClientID, "name": e.Name})
	}
	c.JSON(http.StatusOK, gin.H{"fans": out})
}

// CompleteKYC marks the user as KYC complete and releases any PENDING_KYC interactions (sends request to companion).
// Call after client completes identity verification. Companion-only requests that were paid but held for KYC are now sent.
func (h *MeHandler) CompleteKYC(c *gin.Context) {
	userID := middleware.GetUserID(c)
	u, err := h.userRepo.GetByID(userID)
	if err != nil || u == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	u.KYC = true
	if err := h.userRepo.Update(u); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	list, err := h.interactionRepo.ListPendingKycByClientID(userID, 50)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "kyc": true, "released": 0})
		return
	}
	clientName := u.Email
	if u.Username != "" {
		clientName = u.Username
	}
	released := 0
	for i := range list {
		ir := &list[i]
		if ir.Payment == nil || ir.Payment.Status != "COMPLETED" {
			continue
		}
		serviceType := ir.InteractionType
		if ir.Payment.Metadata != "" {
			var meta struct {
				ServiceType string `json:"service_type"`
			}
			if json.Unmarshal([]byte(ir.Payment.Metadata), &meta) == nil && meta.ServiceType != "" {
				serviceType = meta.ServiceType
			}
		}
		comp, _ := h.companionRepo.GetByID(ir.CompanionID)
		// If the companion is no longer accepting new requests, cancel and refund the client.
		if comp == nil || !comp.AcceptNewRequests || !comp.IsAvailable {
			ir.Status = domain.RequestStatusRejected
			_ = h.interactionRepo.Update(ir)
			if h.walletRepo != nil {
				amtCents := ir.Payment.AmountCents
				_ = h.walletRepo.Credit(userID, amtCents)
				_ = h.walletRepo.RecordTransaction(userID, amtCents, domain.WalletTxTypeRefund, fmt.Sprintf("interaction_%d", ir.ID))
			}
			companionName := "your companion"
			if comp != nil {
				companionName = comp.DisplayName
			}
			if h.notifSvc != nil {
				_ = h.notifSvc.Notify(userID, "KYC_REFUND", "Companion unavailable",
					companionName+" is no longer available. Your payment has been refunded to your wallet.",
					map[string]interface{}{"interaction_id": ir.ID})
			}
			continue
		}
		ir.Status = domain.RequestStatusPending
		if err := h.interactionRepo.Update(ir); err != nil {
			continue
		}
		released++
		// KYC now done: mark companion unavailable (request is now active)
		comp.IsAvailable = false
		_ = h.companionRepo.Update(comp)
		if h.notifSvc != nil {
			_ = h.notifSvc.NotifyPaidRequest(comp.UserID, ir.ID, clientName, serviceType)
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "kyc": true, "released": released})
}

// CompleteOnboarding marks companion onboarding as complete. Optional date_of_birth for Google signups.
func (h *MeHandler) CompleteOnboarding(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role, _ := c.Get("role")
	roleStr, _ := role.(string)
	if roleStr != domain.RoleCompanion && roleStr != domain.RoleClient {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid role"})
		return
	}

	var req struct {
		DateOfBirth string `json:"date_of_birth"` // optional, for Google signups missing DOB
	}
	_ = c.ShouldBindJSON(&req)

	u, err := h.userRepo.GetByID(userID)
	if err != nil || u == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if req.DateOfBirth != "" && u.DateOfBirth == nil {
		if dob, err := time.Parse("2006-01-02", req.DateOfBirth); err == nil {
			u.DateOfBirth = &dob
			_ = h.userRepo.Update(u)
		}
	}

	if roleStr == domain.RoleCompanion {
		profile, err := h.companionRepo.GetByUserID(userID)
		if err != nil || profile == nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "companion profile required"})
			return
		}
		now := time.Now()
		profile.OnboardingCompletedAt = &now
		if err := h.companionRepo.Update(profile); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
