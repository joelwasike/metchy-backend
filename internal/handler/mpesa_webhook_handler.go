package handler

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"lusty/internal/models"
	"lusty/internal/repository"
	"lusty/internal/service"

	"github.com/gin-gonic/gin"
)

// LiberecMpesaCallback is the webhook payload from TheLiberec after M-Pesa payment.
type LiberecMpesaCallback struct {
	Amount                  string `json:"amount"`
	BalanceCredited         bool   `json:"balance_credited"`
	CheckoutRequestID       string `json:"checkout_request_id"`
	Currency                string `json:"currency"`
	CustomerPhone           string `json:"customer_phone"`
	MerchantID              int    `json:"merchant_id"`
	MerchantOrderID         string `json:"merchant_order_id"`
	MerchantRequestID       string `json:"merchant_request_id"`
	OrderID                 string `json:"order_id"`
	PaymentMethod           string `json:"payment_method"`
	ReceiptNumber           string `json:"receipt_number"`
	ReferenceOrderID        string `json:"reference_order_id"`
	Status                  string `json:"status"`
	StatusCode              string `json:"status_code"`
	StatusDescription       string `json:"status_description"`
	TransactionDate         string `json:"transaction_date"`
	TransactionType         string `json:"transaction_type"`
	TransactionUUID         string `json:"transaction_uuid"`
	UpdatedAt               string `json:"updated_at"`
}

type MpesaWebhookHandler struct {
	paymentRepo     *repository.PaymentRepository
	interactionRepo *repository.InteractionRepository
	companionRepo   *repository.CompanionRepository
	walletRepo      *repository.WalletRepository
	auditRepo       *repository.AuditLogRepository
	notifSvc        *service.NotificationService
}

func NewMpesaWebhookHandler(
	paymentRepo *repository.PaymentRepository,
	interactionRepo *repository.InteractionRepository,
	companionRepo *repository.CompanionRepository,
	walletRepo *repository.WalletRepository,
	auditRepo *repository.AuditLogRepository,
	notifSvc *service.NotificationService,
) *MpesaWebhookHandler {
	return &MpesaWebhookHandler{
		paymentRepo:     paymentRepo,
		interactionRepo: interactionRepo,
		companionRepo:   companionRepo,
		walletRepo:      walletRepo,
		auditRepo:       auditRepo,
		notifSvc:        notifSvc,
	}
}

// Handle processes TheLiberec M-Pesa callback. On status=COMPLETED: marks payment done, notifies companion to accept/deny. Companion must accept before chat unlocks.
func (h *MpesaWebhookHandler) Handle(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("[MPESA callback] ReadBody error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	log.Printf("[MPESA callback] raw body: %s", string(body))
	var payload LiberecMpesaCallback
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("[MPESA callback] json unmarshal error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	log.Printf("[MPESA callback] parsed: status=%s merchant_order_id=%s order_id=%s reference_order_id=%s amount=%s", payload.Status, payload.MerchantOrderID, payload.OrderID, payload.ReferenceOrderID, payload.Amount)
	orderID := payload.MerchantOrderID
	if orderID == "" {
		orderID = payload.OrderID
	}
	if orderID == "" {
		orderID = payload.ReferenceOrderID
	}
	if orderID == "" {
		log.Printf("[MPESA callback] no order_id in payload, acknowledging")
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}
	p, err := h.paymentRepo.GetByProviderRef(orderID)
	if err != nil || p == nil {
		log.Printf("[MPESA callback] payment not found for order_id=%s", orderID)
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}
	if p.Status == "COMPLETED" {
		log.Printf("[MPESA callback] payment %d already COMPLETED for order_id=%s", p.ID, orderID)
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}
	if payload.Status != "COMPLETED" {
		log.Printf("[MPESA callback] non-COMPLETED status=%s status_code=%s for order_id=%s, refunding wallet if any", payload.Status, payload.StatusCode, orderID)
		// M-Pesa failed/cancelled - refund any wallet portion
		if p.Metadata != "" {
			var meta struct {
				WalletCents int64 `json:"wallet_cents"`
			}
			if json.Unmarshal([]byte(p.Metadata), &meta) == nil && meta.WalletCents > 0 {
				_ = h.walletRepo.Credit(p.UserID, meta.WalletCents)
			}
		}
		if p.Status == "PENDING" {
			p.Status = "FAILED"
			_ = h.paymentRepo.Update(p)
		}
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}
	log.Printf("[MPESA callback] marking payment %d COMPLETED for order_id=%s", p.ID, orderID)
	now := time.Now()
	p.Status = "COMPLETED"
	p.CompletedAt = &now
	if err := h.paymentRepo.Update(p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	_ = h.notifSvc.NotifyPaymentConfirmed(p.UserID, p.AmountCents, orderID)
	_ = h.auditRepo.Create(&models.AuditLog{
		UserID:     &p.UserID,
		Action:     "mpesa_payment_completed",
		Resource:   "payment",
		ResourceID: orderID,
		IP:         c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
	})

	// Boost payment: no interaction, create CompanionBoost for 24h
	var meta struct {
		Type string `json:"type"`
	}
	if p.Metadata != "" {
		_ = json.Unmarshal([]byte(p.Metadata), &meta)
	}
	if meta.Type == "BOOST" {
		profile, _ := h.companionRepo.GetByUserID(p.UserID)
		if profile != nil {
			boostEnd := now.Add(24 * time.Hour)
			b := &models.CompanionBoost{
				CompanionID: profile.ID,
				BoostType:   "24h",
				StartAt:     now,
				EndAt:       boostEnd,
				IsActive:    true,
			}
			_ = h.companionRepo.CreateBoost(b)
			log.Printf("[MPESA callback] boost activated for companion %d (payment %d)", profile.ID, p.ID)
		}
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}

	// Payment confirmed. Companion must explicitly accept before chat unlocks.
	// If companion already rejected: refund client's wallet.
	ir, err := h.interactionRepo.GetByPaymentID(p.ID)
	if err == nil && ir != nil {
		if ir.Status == "REJECTED" {
			if ir.Payment != nil {
				_ = h.walletRepo.Credit(ir.ClientID, ir.Payment.AmountCents)
				log.Printf("[MPESA callback] interaction %d already REJECTED, refunded %d cents to client %d", ir.ID, ir.Payment.AmountCents, ir.ClientID)
			}
		} else if ir.Status == "PENDING" {
			// Notify companion: client has paid, they should accept or deny
			clientName := "A client"
			if ir.Client.ID != 0 {
				if ir.Client.Username != "" {
					clientName = ir.Client.Username
				} else {
					clientName = ir.Client.Email
				}
			}
			serviceType := ir.InteractionType
			if p.Metadata != "" {
				var meta struct {
					ServiceType string `json:"service_type"`
				}
				if json.Unmarshal([]byte(p.Metadata), &meta) == nil && meta.ServiceType != "" {
					serviceType = meta.ServiceType
				}
			}
			comp, _ := h.companionRepo.GetByID(ir.CompanionID)
			if comp != nil {
				_ = h.notifSvc.NotifyPaidRequest(comp.UserID, ir.ID, clientName, serviceType)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}
