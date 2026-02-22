package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"lusty/internal/domain"
	"lusty/internal/models"
	"lusty/internal/repository"
	"lusty/internal/service"
)

type CryptoWebhookHandler struct {
	paymentRepo     *repository.PaymentRepository
	interactionRepo *repository.InteractionRepository
	companionRepo   *repository.CompanionRepository
	walletRepo      *repository.WalletRepository
	userRepo        *repository.UserRepository
	notifSvc        *service.NotificationService
	referralRepo    *repository.ReferralRepository
}

func NewCryptoWebhookHandler(
	paymentRepo *repository.PaymentRepository,
	interactionRepo *repository.InteractionRepository,
	companionRepo *repository.CompanionRepository,
	walletRepo *repository.WalletRepository,
	userRepo *repository.UserRepository,
	notifSvc *service.NotificationService,
	referralRepo *repository.ReferralRepository,
) *CryptoWebhookHandler {
	return &CryptoWebhookHandler{
		paymentRepo:     paymentRepo,
		interactionRepo: interactionRepo,
		companionRepo:   companionRepo,
		walletRepo:      walletRepo,
		userRepo:        userRepo,
		notifSvc:        notifSvc,
		referralRepo:    referralRepo,
	}
}

type swapuziCallback struct {
	Event             string  `json:"event"`
	MerchantID        int     `json:"merchant_id"`
	DepositID         int     `json:"deposit_id"`
	MerchantDepositID string  `json:"merchant_deposit_id"`
	SolanaAddress     string  `json:"solana_address"`
	ReceivedAmount    float64 `json:"received_amount"`
	ExpectedAmount    float64 `json:"expected_amount"`
	Status            string  `json:"status"`
	Timestamp         int64   `json:"timestamp"`
}

// Handle processes Swapuzi Solana deposit callbacks.
// On status=completed: marks payment done, creates interaction, notifies companion.
// On status=expired/failed: marks payment failed, refunds any wallet portion.
func (h *CryptoWebhookHandler) Handle(c *gin.Context) {
	var payload swapuziCallback
	if err := c.ShouldBindJSON(&payload); err != nil {
		log.Printf("[Crypto webhook] invalid payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	log.Printf("[Crypto webhook] event=%s merchant_deposit_id=%s status=%s received=%.4f expected=%.4f",
		payload.Event, payload.MerchantDepositID, payload.Status, payload.ReceivedAmount, payload.ExpectedAmount)

	if payload.MerchantDepositID == "" {
		log.Printf("[Crypto webhook] no merchant_deposit_id in payload, acknowledging")
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}

	// Look up payment by our order_id (stored as ProviderRef)
	p, err := h.paymentRepo.GetByProviderRef(payload.MerchantDepositID)
	if err != nil || p == nil {
		log.Printf("[Crypto webhook] payment not found for merchant_deposit_id=%s", payload.MerchantDepositID)
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}
	if p.Status == "COMPLETED" || p.Status == "FAILED" || p.Status == "CANCELLED" {
		log.Printf("[Crypto webhook] payment %d already %s — ignoring", p.ID, p.Status)
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}

	// Expired or failed: refund any wallet portion
	if payload.Status == "expired" || payload.Status == "failed" || payload.Status == "cancelled" {
		p.Status = "FAILED"
		_ = h.paymentRepo.Update(p)
		if p.Metadata != "" {
			var meta struct {
				WalletCents int64 `json:"wallet_cents"`
			}
			if json.Unmarshal([]byte(p.Metadata), &meta) == nil && meta.WalletCents > 0 {
				_ = h.walletRepo.Credit(p.UserID, meta.WalletCents)
			}
		}
		log.Printf("[Crypto webhook] payment %d marked FAILED (event=%s status=%s)", p.ID, payload.Event, payload.Status)
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}

	if payload.Status != "completed" {
		log.Printf("[Crypto webhook] unhandled status=%s for payment %d — ignoring", payload.Status, p.ID)
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}

	// Payment completed — mark COMPLETED
	now := time.Now()
	p.Status = "COMPLETED"
	p.CompletedAt = &now
	if err := h.paymentRepo.Update(p); err != nil {
		log.Printf("[Crypto webhook] failed to mark payment %d COMPLETED: %v", p.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	_ = h.notifSvc.NotifyPaymentConfirmed(p.UserID, p.AmountCents, payload.MerchantDepositID)

	// Referral commission: 5% for the referrer on first 2 orders
	if h.referralRepo != nil {
		clientUser, _ := h.userRepo.GetByID(p.UserID)
		if clientUser != nil && clientUser.IsClient() {
			ref, err := h.referralRepo.GetReferralByReferredUserID(p.UserID)
			if err == nil && ref != nil && ref.CompletedCount < domain.ReferralMaxTransactions {
				commission := int64(float64(p.AmountCents) * domain.ReferralCommissionRate)
				if commission > 0 {
					_ = h.walletRepo.Credit(ref.ReferrerID, commission)
					_ = h.walletRepo.RecordTransaction(ref.ReferrerID, commission, domain.WalletTxTypeReferralCommission,
						fmt.Sprintf("ref_%d_payment_%d", ref.ID, p.ID))
					_ = h.referralRepo.IncrementCompletedCount(ref.ID)
					log.Printf("[Crypto webhook] referral commission %d cents → referrer %d (ref %d)", commission, ref.ReferrerID, ref.ID)
				}
			}
		}
	}

	// Parse metadata to build the interaction request
	var meta struct {
		CompanionID     uint   `json:"companion_id"`
		InteractionType string `json:"interaction_type"`
		ServiceType     string `json:"service_type"`
		WalletCents     int64  `json:"wallet_cents"`
		DurationMinutes int    `json:"duration_minutes"`
	}
	if p.Metadata != "" {
		_ = json.Unmarshal([]byte(p.Metadata), &meta)
	}
	if meta.CompanionID == 0 {
		log.Printf("[Crypto webhook] payment %d has no companion_id in metadata — cannot create interaction", p.ID)
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}

	companion, _ := h.companionRepo.GetByID(meta.CompanionID)
	if companion == nil {
		log.Printf("[Crypto webhook] companion %d not found for payment %d", meta.CompanionID, p.ID)
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}

	clientUser, _ := h.userRepo.GetByID(p.UserID)
	status := domain.RequestStatusPending
	if clientUser != nil && !clientUser.KYC {
		status = "PENDING_KYC"
	}
	durationMinutes := meta.DurationMinutes
	if durationMinutes <= 0 {
		durationMinutes = 1440
	}
	expiresAt := now.Add(30 * time.Minute)
	ir := &models.InteractionRequest{
		ClientID:        p.UserID,
		CompanionID:     meta.CompanionID,
		InteractionType: meta.InteractionType,
		PaymentID:       &p.ID,
		Status:          status,
		DurationMinutes: durationMinutes,
		ExpiresAt:       &expiresAt,
	}
	if err := h.interactionRepo.Create(ir); err != nil {
		log.Printf("[Crypto webhook] failed to create interaction for payment %d: %v", p.ID, err)
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}

	if status == "PENDING_KYC" {
		log.Printf("[Crypto webhook] payment %d: interaction %d set PENDING_KYC (client %d KYC not complete)", p.ID, ir.ID, p.UserID)
	} else {
		// KYC done: mark companion unavailable and notify
		companion.IsAvailable = false
		_ = h.companionRepo.Update(companion)
		clientName := "A client"
		if clientUser != nil {
			if clientUser.Username != "" {
				clientName = clientUser.Username
			} else {
				clientName = clientUser.Email
			}
		}
		serviceType := meta.InteractionType
		if meta.ServiceType != "" {
			serviceType = meta.ServiceType
		}
		_ = h.notifSvc.NotifyPaidRequest(companion.UserID, ir.ID, clientName, serviceType)
		log.Printf("[Crypto webhook] payment %d: interaction %d created, companion %d notified", p.ID, ir.ID, meta.CompanionID)
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}
