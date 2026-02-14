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

// Handle processes TheLiberec M-Pesa callback. On status=COMPLETED: marks payment done, accepts interaction, creates chat session, unlocks chat/video.
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

	// Auto-accept interaction when payment completes: set ACCEPTED, create chat session
	// If companion already rejected: refund client's wallet instead
	ir, err := h.interactionRepo.GetByPaymentID(p.ID)
	if err == nil && ir != nil {
		if ir.Status == "REJECTED" {
			// Companion rejected before webhook arrived: refund client to wallet
			if ir.Payment != nil {
				_ = h.walletRepo.Credit(ir.ClientID, ir.Payment.AmountCents)
				log.Printf("[MPESA callback] interaction %d already REJECTED, refunded %d cents to client %d", ir.ID, ir.Payment.AmountCents, ir.ClientID)
			}
		} else if ir.Status == "PENDING" {
			now := time.Now()
			ir.Status = "ACCEPTED"
			ir.AcceptedAt = &now
			if err := h.interactionRepo.Update(ir); err != nil {
				log.Printf("[MPESA callback] auto-accept update failed: %v", err)
			} else {
				endsAt := now.Add(time.Duration(ir.DurationMinutes) * time.Minute)
				if ir.DurationMinutes <= 0 {
					endsAt = now.Add(24 * time.Hour)
				}
				session := &models.ChatSession{InteractionID: ir.ID, StartedAt: now, EndsAt: endsAt}
				if err := h.interactionRepo.CreateChatSession(session); err != nil {
					log.Printf("[MPESA callback] create chat session failed: %v", err)
				} else {
					companionName := "Companion"
					comp, _ := h.companionRepo.GetByID(ir.CompanionID)
					if comp != nil {
						companionName = comp.DisplayName
						// Credit companion's wallet (balance shown; withdrawable after client confirms service done)
						amountCents := p.AmountCents
						_ = h.walletRepo.Credit(comp.UserID, amountCents)
						log.Printf("[MPESA callback] credited companion %d wallet %d cents", comp.UserID, amountCents)
					}
					_ = h.notifSvc.NotifyAccepted(ir.ClientID, companionName, ir.ID)
					log.Printf("[MPESA callback] auto-accepted interaction %d, chat session created", ir.ID)
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}
