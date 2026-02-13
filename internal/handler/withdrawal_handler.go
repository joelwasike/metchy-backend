package handler

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"lusty/config"
	"lusty/internal/domain"
	"lusty/internal/middleware"
	"lusty/internal/models"
	"lusty/internal/repository"
	"lusty/pkg/payment"

	"github.com/gin-gonic/gin"
)

type WithdrawalHandler struct {
	cfg             *config.Config
	walletRepo      *repository.WalletRepository
	withdrawalRepo  *repository.WithdrawalRepository
	companionRepo   *repository.CompanionRepository
	mpesaProvider   *payment.LiberecMpesaProvider
}

func NewWithdrawalHandler(
	cfg *config.Config,
	walletRepo *repository.WalletRepository,
	withdrawalRepo *repository.WithdrawalRepository,
	companionRepo *repository.CompanionRepository,
) *WithdrawalHandler {
	h := &WithdrawalHandler{
		cfg:            cfg,
		walletRepo:     walletRepo,
		withdrawalRepo: withdrawalRepo,
		companionRepo:  companionRepo,
	}
	h.mpesaProvider = payment.NewLiberecMpesaProvider(
		cfg.LiberecMpesa.BaseURL,
		cfg.LiberecMpesa.Email,
		cfg.LiberecMpesa.Password,
		cfg.LiberecMpesa.WebhookBaseURL,
	)
	return h
}

// Create initiates a withdrawal to M-Pesa (B2C). Companion only.
func (h *WithdrawalHandler) Create(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role, _ := c.Get("role")
	if roleStr, _ := role.(string); roleStr != domain.RoleCompanion {
		c.JSON(http.StatusForbidden, gin.H{"error": "companion only"})
		return
	}
	var req struct {
		AmountKES   int64  `json:"amount_kes" binding:"required,min=1"`
		PhoneNumber string `json:"phone_number" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	phone := normalizePhone(req.PhoneNumber)
	if phone == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid phone number"})
		return
	}
	amountCents := req.AmountKES * 100
	wallet, err := h.walletRepo.GetOrCreate(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "wallet error"})
		return
	}
	if wallet.WithdrawableCents < amountCents {
		c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient withdrawable balance"})
		return
	}
	orderID := fmt.Sprintf("wd-%s", uuid.New().String())
	callbackURL := ""
	if h.cfg.LiberecMpesa.WebhookBaseURL != "" {
		base := h.cfg.LiberecMpesa.WebhookBaseURL
		if len(base) > 0 && base[0] != 'h' {
			base = "https://" + base
		}
		callbackURL = base + "/api/v1/webhooks/withdrawal"
	}
	b2cReq := payment.B2CRequest{
		Amount:      req.AmountKES,
		PhoneNumber: phone,
		Description: "Withdrawal from Metchi",
		Remarks:     "Companion withdrawal",
		OrderID:     orderID,
		CallbackURL: callbackURL,
	}
	resp, err := h.mpesaProvider.InitiateB2C(c.Request.Context(), b2cReq)
	if err != nil {
		log.Printf("[Withdrawal] B2C init failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "withdrawal init failed: " + err.Error()})
		return
	}
	if err := h.walletRepo.DebitWithdrawable(userID, amountCents); err != nil {
		log.Printf("[Withdrawal] debit failed after B2C: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to deduct balance"})
		return
	}
	w := &models.Withdrawal{
		UserID:      userID,
		OrderID:     orderID,
		AmountCents: amountCents,
		PhoneNumber: phone,
		Status:      "PENDING",
		ProviderRef: resp.UUID,
	}
	if err := h.withdrawalRepo.Create(w); err != nil {
		h.walletRepo.CreditWithdrawable(userID, amountCents)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record withdrawal"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"id":           w.ID,
		"order_id":     orderID,
		"amount_kes":   req.AmountKES,
		"phone_number": phone,
		"status":       "PENDING",
		"message":      "Withdrawal initiated. Check your phone for confirmation.",
	})
}

func normalizePhone(s string) string {
	s = regexp.MustCompile(`\D`).ReplaceAllString(s, "")
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "0") {
		s = "254" + s[1:]
	} else if !strings.HasPrefix(s, "254") {
		s = "254" + s
	}
	if len(s) != 12 {
		return ""
	}
	return s
}
