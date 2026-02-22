package handler

import (
	"fmt"
	"math"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lusty/config"
	"lusty/internal/middleware"
	"lusty/internal/models"
	"lusty/internal/repository"
	"lusty/internal/service"
	"lusty/pkg/payment"
)

type CryptoHandler struct {
	cfg           *config.Config
	paymentRepo   *repository.PaymentRepository
	companionRepo *repository.CompanionRepository
	walletRepo    *repository.WalletRepository
	userRepo      *repository.UserRepository
	notifSvc      *service.NotificationService
	swapuzi       *payment.SwapuziProvider
}

func NewCryptoHandler(
	cfg *config.Config,
	paymentRepo *repository.PaymentRepository,
	companionRepo *repository.CompanionRepository,
	walletRepo *repository.WalletRepository,
	userRepo *repository.UserRepository,
	notifSvc *service.NotificationService,
) *CryptoHandler {
	return &CryptoHandler{
		cfg:           cfg,
		paymentRepo:   paymentRepo,
		companionRepo: companionRepo,
		walletRepo:    walletRepo,
		userRepo:      userRepo,
		notifSvc:      notifSvc,
		swapuzi:       payment.NewSwapuziProvider(cfg.Swapuzi.BaseURL, cfg.Swapuzi.Email, cfg.Swapuzi.Password),
	}
}

// GetRates returns the current USDT/KES exchange rates proxied from Swapuzi.
func (h *CryptoHandler) GetRates(c *gin.Context) {
	rates, err := h.swapuzi.GetRates(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch exchange rates"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"usdt_buying_rate":  rates.UsdtBuyingRate,
		"usdt_selling_rate": rates.UsdtSellingRate,
	})
}

// Initiate starts a Solana/USDT deposit for a companion service.
// It creates a PENDING payment record and returns the Swapuzi page URL for the client to complete payment.
// The interaction request is created only after the webhook confirms payment.
func (h *CryptoHandler) Initiate(c *gin.Context) {
	clientID := middleware.GetUserID(c)
	var req struct {
		CompanionID     uint   `json:"companion_id" binding:"required"`
		InteractionType string `json:"interaction_type" binding:"required,oneof=CHAT VIDEO BOOKING"`
		ServiceType     string `json:"service_type"`
		AmountKES       int64  `json:"amount_kes" binding:"required,min=1"`
		WalletAmountKES int64  `json:"wallet_amount_kes"`
		DurationMinutes int    `json:"duration_minutes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.WalletAmountKES < 0 {
		req.WalletAmountKES = 0
	}
	if req.WalletAmountKES > req.AmountKES {
		req.WalletAmountKES = req.AmountKES
	}

	companion, err := h.companionRepo.GetByID(req.CompanionID)
	if err != nil || companion == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "companion not found"})
		return
	}

	amountCents := req.AmountKES * 100
	walletCents := req.WalletAmountKES * 100
	cryptoCents := amountCents - walletCents // portion to be paid via USDT

	// Deduct wallet portion upfront
	if walletCents > 0 {
		if err := h.walletRepo.Debit(clientID, walletCents); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient wallet balance"})
			return
		}
	}

	// Fetch current USDT rate to convert KES → USDT
	rates, err := h.swapuzi.GetRates(c.Request.Context())
	if err != nil {
		if walletCents > 0 {
			_ = h.walletRepo.Credit(clientID, walletCents)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch exchange rates"})
		return
	}
	if rates.UsdtBuyingRate <= 0 {
		if walletCents > 0 {
			_ = h.walletRepo.Credit(clientID, walletCents)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid exchange rate received"})
		return
	}

	cryptoKES := float64(cryptoCents) / 100.0
	// Round up to 4 decimal places so we never under-request
	usdtAmount := math.Ceil(cryptoKES/rates.UsdtBuyingRate*10000) / 10000

	orderID := fmt.Sprintf("metchi-sol-%s", uuid.New().String())
	webhookURL := ""
	if h.cfg.Swapuzi.WebhookBaseURL != "" {
		webhookURL = h.cfg.Swapuzi.WebhookBaseURL + "/api/v1/webhooks/crypto"
	}

	durationMinutes := req.DurationMinutes
	if durationMinutes <= 0 {
		durationMinutes = 1440 // 24 hours
	}

	// Save payment as PENDING; metadata carries all info needed by the webhook to create the interaction
	meta := fmt.Sprintf(
		`{"companion_id":%d,"interaction_type":%q,"service_type":%q,"wallet_cents":%d,"duration_minutes":%d}`,
		req.CompanionID, req.InteractionType, req.ServiceType, walletCents, durationMinutes,
	)
	pay := &models.Payment{
		UserID:         clientID,
		AmountCents:    amountCents,
		Currency:       "KES",
		Provider:       "solana",
		ProviderRef:    orderID,
		Status:         "PENDING",
		IdempotencyKey: orderID,
		Metadata:       meta,
	}
	if err := h.paymentRepo.Create(pay); err != nil {
		if walletCents > 0 {
			_ = h.walletRepo.Credit(clientID, walletCents)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "payment record creation failed"})
		return
	}

	// Initiate Swapuzi deposit
	notes := fmt.Sprintf("Payment for %s with %s", req.InteractionType, companion.DisplayName)
	deposit, err := h.swapuzi.InitiateDeposit(c.Request.Context(), orderID, webhookURL, notes, usdtAmount)
	if err != nil {
		pay.Status = "FAILED"
		_ = h.paymentRepo.Update(pay)
		if walletCents > 0 {
			_ = h.walletRepo.Credit(clientID, walletCents)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to initiate crypto payment: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"order_id":       orderID,
		"page_url":       deposit.PageURL,
		"amount_kes":     req.AmountKES,
		"amount_usdt":    usdtAmount,
		"currency":       "KES",
		"payment_status": "PENDING",
		"expires_at":     deposit.ExpiresAt,
		"message":        "Complete your USDT payment at the provided URL. You'll be notified once confirmed.",
	})
}
