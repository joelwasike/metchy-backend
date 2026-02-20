package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"lusty/config"
	"lusty/internal/domain"
	"lusty/internal/middleware"
	"lusty/internal/models"
	"lusty/internal/repository"
	"lusty/internal/service"
	"lusty/pkg/payment"

	"github.com/gin-gonic/gin"
)

type MpesaHandler struct {
	cfg             *config.Config
	paymentRepo     *repository.PaymentRepository
	interactionRepo *repository.InteractionRepository
	companionRepo   *repository.CompanionRepository
	walletRepo      *repository.WalletRepository
	userRepo        *repository.UserRepository
	notifSvc        *service.NotificationService
	mpesaProvider   payment.Provider
}

func NewMpesaHandler(
	cfg *config.Config,
	paymentRepo *repository.PaymentRepository,
	interactionRepo *repository.InteractionRepository,
	companionRepo *repository.CompanionRepository,
	walletRepo *repository.WalletRepository,
	userRepo *repository.UserRepository,
	notifSvc *service.NotificationService,
) *MpesaHandler {
	m := &MpesaHandler{
		cfg:             cfg,
		paymentRepo:     paymentRepo,
		interactionRepo: interactionRepo,
		companionRepo:   companionRepo,
		walletRepo:      walletRepo,
		userRepo:       userRepo,
		notifSvc:        notifSvc,
	}
	m.mpesaProvider = payment.NewLiberecMpesaProvider(
		cfg.LiberecMpesa.BaseURL,
		cfg.LiberecMpesa.Email,
		cfg.LiberecMpesa.Password,
		cfg.LiberecMpesa.WebhookBaseURL,
	)
	return m
}

// Initiate starts payment: wallet-only (instant) or M-Pesa STK (or wallet + M-Pesa partial).
// wallet_amount_kes: amount to deduct from wallet. Rest paid via M-Pesa. If total covered by wallet, no M-Pesa.
func (h *MpesaHandler) Initiate(c *gin.Context) {
	clientID := middleware.GetUserID(c)
	var req struct {
		CompanionID       uint   `json:"companion_id" binding:"required"`
		InteractionType   string `json:"interaction_type" binding:"required,oneof=CHAT VIDEO BOOKING"`
		ServiceType       string `json:"service_type"` // HOOKUP/SEX, MASSAGE, NUDE_VIDEO_CALL, SHAVING_WAXING, etc.
		AmountKES         int64  `json:"amount_kes" binding:"required,min=1"`
		WalletAmountKES   int64  `json:"wallet_amount_kes"` // optional: use from wallet
		CustomerPhone     string `json:"customer_phone"`
		CustomerFirstName string `json:"customer_first_name"`
		CustomerLastName  string `json:"customer_last_name"`
		CustomerEmail     string `json:"customer_email"`
		DurationMinutes   int    `json:"duration_minutes"`
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
	mpesaCents := amountCents - walletCents

	// Wallet-only: deduct and create completed payment + request immediately
	if mpesaCents <= 0 {
		if err := h.walletRepo.Debit(clientID, walletCents); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient wallet balance"})
			return
		}
		orderID := fmt.Sprintf("lusty-w-%s", uuid.New().String())
		walletOnlyMeta := ""
		if req.ServiceType != "" {
			walletOnlyMeta = fmt.Sprintf(`{"service_type":%q}`, req.ServiceType)
		}
		pay := &models.Payment{
			UserID:         clientID,
			AmountCents:    amountCents,
			Currency:       "KES",
			Provider:       "wallet",
			ProviderRef:    orderID,
			Status:         "COMPLETED",
			IdempotencyKey: orderID,
			Metadata:       walletOnlyMeta,
		}
		now := time.Now()
		pay.CompletedAt = &now
		if err := h.paymentRepo.Create(pay); err != nil {
			h.walletRepo.Credit(clientID, walletCents) // rollback
			c.JSON(http.StatusInternalServerError, gin.H{"error": "payment create failed"})
			return
		}
		expiresAt := now.Add(30 * time.Minute)
		clientUser, _ := h.userRepo.GetByID(clientID)
		status := domain.RequestStatusPending
		if clientUser != nil && !clientUser.KYC {
			status = "PENDING_KYC" // request not sent to companion until KYC complete
		}
		ir := &models.InteractionRequest{
			ClientID:         clientID,
			CompanionID:      req.CompanionID,
			InteractionType:  req.InteractionType,
			PaymentID:        &pay.ID,
			Status:           status,
			DurationMinutes:  req.DurationMinutes,
			ExpiresAt:        &expiresAt,
		}
		if ir.DurationMinutes <= 0 {
			ir.DurationMinutes = 1440 // 24 hours
		}
		if err := h.interactionRepo.Create(ir); err != nil {
			h.walletRepo.Credit(clientID, walletCents)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "interaction create failed"})
			return
		}
		msg := "Payment successful! Waiting for " + companion.DisplayName + " to accept your request."
		if status == "PENDING_KYC" {
			msg = "Payment successful! Complete KYC to send your request to " + companion.DisplayName + "."
		} else {
			clientName := "A client"
			if clientUser != nil {
				if clientUser.Username != "" {
					clientName = clientUser.Username
				} else {
					clientName = clientUser.Email
				}
			}
			_ = h.notifSvc.NotifyPaidRequest(companion.UserID, ir.ID, clientName, req.InteractionType)
		}
		c.JSON(http.StatusOK, gin.H{
			"order_id":        orderID,
			"interaction_id":  ir.ID,
			"amount":          req.AmountKES,
			"currency":        "KES",
			"payment_status":  "COMPLETED",
			"message":         msg,
			"requires_kyc":   status == "PENDING_KYC",
		})
		return
	}

	// M-Pesa (with optional wallet portion)
	if req.CustomerPhone == "" || req.CustomerFirstName == "" || req.CustomerLastName == "" || req.CustomerEmail == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "customer_phone, customer_first_name, customer_last_name, customer_email required for M-Pesa"})
		return
	}
	if walletCents > 0 {
		if err := h.walletRepo.Debit(clientID, walletCents); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient wallet balance"})
			return
		}
	}
	orderID := fmt.Sprintf("lusty-%s", uuid.New().String())
	callbackURL := ""
	if h.cfg.LiberecMpesa.WebhookBaseURL != "" {
		callbackURL = h.cfg.LiberecMpesa.WebhookBaseURL + "/api/v1/webhooks/mpesa"
	}
	log.Printf("[MPESA] Initiate order_id=%s callback_url=%s amount_kes=%d mpesa_kes=%d", orderID, callbackURL, req.AmountKES, mpesaCents/100)
	walletMeta := ""
	if walletCents > 0 || req.ServiceType != "" {
		walletMeta = fmt.Sprintf(`{"wallet_cents":%d,"service_type":%q}`, walletCents, req.ServiceType)
	}
	pay := &models.Payment{
		UserID:         clientID,
		AmountCents:    amountCents,
		Currency:       "KES",
		Provider:       "mpesa_liberec",
		ProviderRef:    orderID,
		Status:         "PENDING",
		IdempotencyKey: orderID,
		Metadata:       walletMeta,
	}
	if err := h.paymentRepo.Create(pay); err != nil {
		if walletCents > 0 {
			h.walletRepo.Credit(clientID, walletCents)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "payment create failed"})
		return
	}
	stkReq := payment.PaymentRequest{
		UserID:            clientID,
		AmountCents:       mpesaCents,
		Currency:          "KES",
		OrderID:           orderID,
		CustomerPhone:     req.CustomerPhone,
		CustomerFirstName: req.CustomerFirstName,
		CustomerLastName:  req.CustomerLastName,
		CustomerEmail:     req.CustomerEmail,
		CallbackURL:       callbackURL,
		Description:       fmt.Sprintf("Payment for %s", req.InteractionType),
	}
	if reqJSON, _ := json.Marshal(stkReq); reqJSON != nil {
		log.Printf("[MPESA] STK request: %s", string(reqJSON))
	}
	resp, err := h.mpesaProvider.InitiatePayment(c.Request.Context(), stkReq)
	if err != nil {
		log.Printf("[MPESA] InitiatePayment error: %v", err)
		h.paymentRepo.Update(&models.Payment{ID: pay.ID, Status: "FAILED"})
		if walletCents > 0 {
			h.walletRepo.Credit(clientID, walletCents)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "mpesa init failed: " + err.Error()})
		return
	}
	expiresAt := time.Now().Add(30 * time.Minute)
	ir := &models.InteractionRequest{
		ClientID:         clientID,
		CompanionID:      req.CompanionID,
		InteractionType:  req.InteractionType,
		PaymentID:        &pay.ID,
		Status:           domain.RequestStatusPending,
		DurationMinutes: req.DurationMinutes,
		ExpiresAt:        &expiresAt,
	}
	if ir.DurationMinutes <= 0 {
		ir.DurationMinutes = 1440 // 24 hours
	}
	if err := h.interactionRepo.Create(ir); err != nil {
		if walletCents > 0 {
			h.walletRepo.Credit(clientID, walletCents)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "interaction create failed"})
		return
	}
	log.Printf("[MPESA] STK sent order_id=%s checkout_request_id=%s — polling for completion", orderID, resp.CheckoutRequestID)

	// Poll the DB every 2 seconds until the M-Pesa webhook marks the payment COMPLETED or FAILED.
	// Maximum wait is 90 seconds (M-Pesa STK push typically resolves within 60s).
	ctx := c.Request.Context()
	deadline := time.Now().Add(90 * time.Second)
	var finalPayment *models.Payment
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return // client disconnected
		}
		p2, err := h.paymentRepo.GetByID(pay.ID)
		if err == nil && (p2.Status == "COMPLETED" || p2.Status == "FAILED") {
			finalPayment = p2
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}

	if finalPayment == nil || finalPayment.Status != "COMPLETED" {
		// Timed out or failed: cancel the payment so late webhooks are ignored, refund wallet portion.
		pay.Status = "CANCELLED"
		_ = h.paymentRepo.Update(pay)
		ir.Status = domain.RequestStatusExpired
		_ = h.interactionRepo.Update(ir)
		if walletCents > 0 {
			_ = h.walletRepo.Credit(clientID, walletCents)
		}
		resultStatus := "TIMEOUT"
		msg := "Payment timed out. Please try again."
		if finalPayment != nil && finalPayment.Status == "FAILED" {
			resultStatus = "FAILED"
			msg = "Payment was cancelled or failed. Please try again."
		}
		log.Printf("[MPESA] order_id=%s result=%s", orderID, resultStatus)
		c.JSON(http.StatusOK, gin.H{
			"order_id":       orderID,
			"payment_status": resultStatus,
			"message":        msg,
		})
		return
	}

	// Payment COMPLETED — re-read the interaction (webhook may have set PENDING_KYC).
	ir2, err := h.interactionRepo.GetByID(ir.ID)
	if err != nil || ir2 == nil {
		ir2 = ir
	}
	requiresKyc := ir2.Status == "PENDING_KYC"
	msg := "Payment successful! Waiting for " + companion.DisplayName + " to accept your request."
	if requiresKyc {
		msg = "Payment successful! Complete KYC to send your request to " + companion.DisplayName + "."
	}
	log.Printf("[MPESA] order_id=%s COMPLETED interaction_id=%d requires_kyc=%v", orderID, ir2.ID, requiresKyc)
	c.JSON(http.StatusOK, gin.H{
		"order_id":       orderID,
		"interaction_id": ir2.ID,
		"amount":         req.AmountKES,
		"currency":       "KES",
		"payment_status": "COMPLETED",
		"message":        msg,
		"requires_kyc":   requiresKyc,
	})
}

// InitiateBoost starts M-Pesa payment for companion boost (1000 KES, 1 month). Companion only.
func (h *MpesaHandler) InitiateBoost(c *gin.Context) {
	userID := middleware.GetUserID(c)
	profile, err := h.companionRepo.GetByUserID(userID)
	if err != nil || profile == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "companion profile required"})
		return
	}
	var req struct {
		AmountKES         int64  `json:"amount_kes"` // default 1000
		CustomerPhone     string `json:"customer_phone" binding:"required"`
		CustomerFirstName string `json:"customer_first_name" binding:"required"`
		CustomerLastName  string `json:"customer_last_name" binding:"required"`
		CustomerEmail     string `json:"customer_email" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "customer_phone, customer_first_name, customer_last_name, customer_email required"})
		return
	}
	if req.AmountKES <= 0 {
		req.AmountKES = 1000
	}
	amountCents := req.AmountKES * 100
	orderID := fmt.Sprintf("lusty-boost-%s", uuid.New().String())
	callbackURL := ""
	if h.cfg.LiberecMpesa.WebhookBaseURL != "" {
		callbackURL = h.cfg.LiberecMpesa.WebhookBaseURL + "/api/v1/webhooks/mpesa"
	}
	pay := &models.Payment{
		UserID:         userID,
		AmountCents:    amountCents,
		Currency:       "KES",
		Provider:       "mpesa_liberec",
		ProviderRef:    orderID,
		Status:         "PENDING",
		IdempotencyKey: orderID,
		Metadata:       `{"type":"BOOST"}`,
	}
	if err := h.paymentRepo.Create(pay); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "payment create failed"})
		return
	}
	stkReq := payment.PaymentRequest{
		UserID:            userID,
		AmountCents:       amountCents,
		Currency:          "KES",
		OrderID:           orderID,
		CustomerPhone:     req.CustomerPhone,
		CustomerFirstName: req.CustomerFirstName,
		CustomerLastName:  req.CustomerLastName,
		CustomerEmail:     req.CustomerEmail,
		CallbackURL:       callbackURL,
		Description:       "Boost your profile (1 month)",
	}
	resp, err := h.mpesaProvider.InitiatePayment(c.Request.Context(), stkReq)
	if err != nil {
		log.Printf("[MPESA Boost] InitiatePayment error: %v", err)
		h.paymentRepo.Update(&models.Payment{ID: pay.ID, Status: "FAILED"})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "mpesa init failed: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"order_id":            orderID,
		"checkout_request_id": resp.CheckoutRequestID,
		"status":              resp.Status,
		"amount_kes":          req.AmountKES,
		"message":             "Check your phone to complete M-Pesa payment. Boost lasts 1 month.",
	})
}
