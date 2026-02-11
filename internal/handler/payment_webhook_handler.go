package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"lusty/config"
	"lusty/internal/models"
	"lusty/internal/repository"
	"lusty/internal/service"

	"github.com/gin-gonic/gin"
)

type PaymentWebhookHandler struct {
	paymentRepo *repository.PaymentRepository
	auditRepo   *repository.AuditLogRepository
	notifSvc    *service.NotificationService
	cfg         *config.Config
}

func NewPaymentWebhookHandler(paymentRepo *repository.PaymentRepository, auditRepo *repository.AuditLogRepository, notifSvc *service.NotificationService, cfg *config.Config) *PaymentWebhookHandler {
	return &PaymentWebhookHandler{paymentRepo: paymentRepo, auditRepo: auditRepo, notifSvc: notifSvc, cfg: cfg}
}

// Handle is a generic webhook handler. Provider-specific handlers would parse body and verify signature.
// This version expects JSON: { "reference": "...", "status": "COMPLETED" } and optional X-Webhook-Signature.
func (h *PaymentWebhookHandler) Handle(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if h.cfg.Payment.WebhookSecret != "" {
		sig := c.GetHeader("X-Webhook-Signature")
		if !h.verifySignature(body, sig) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
			return
		}
	}
	var payload struct {
		Reference string `json:"reference"`
		Status    string `json:"status"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if payload.Reference == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reference required"})
		return
	}
	p, err := h.paymentRepo.GetByProviderRef(payload.Reference)
	if err != nil || p == nil {
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}
	if p.Status == "COMPLETED" {
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}
	if payload.Status == "COMPLETED" || payload.Status == "completed" {
		now := time.Now()
		p.Status = "COMPLETED"
		p.CompletedAt = &now
		if err := h.paymentRepo.Update(p); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
			return
		}
		_ = h.notifSvc.NotifyPaymentConfirmed(p.UserID, p.AmountCents, p.ProviderRef)
		_ = h.auditRepo.Create(&models.AuditLog{
			UserID:     &p.UserID,
			Action:     "payment_completed",
			Resource:   "payment",
			ResourceID: payload.Reference,
			IP:         c.ClientIP(),
			UserAgent:  c.Request.UserAgent(),
		})
	}
	c.JSON(http.StatusOK, gin.H{"received": true})
}

func (h *PaymentWebhookHandler) verifySignature(body []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(h.cfg.Payment.WebhookSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expected))
}
