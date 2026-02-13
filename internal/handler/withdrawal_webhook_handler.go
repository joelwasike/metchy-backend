package handler

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"lusty/internal/repository"

	"github.com/gin-gonic/gin"
)

// B2CCallback is the webhook payload from M-Pesa B2C.
type B2CCallback struct {
	Amount                  string `json:"amount"`
	BalanceCredited         bool   `json:"balance_credited"`
	CheckoutRequestID       string `json:"checkout_request_id"`
	ConversationID          string `json:"conversation_id"`
	Currency                string `json:"currency"`
	CustomerPhone           string `json:"customer_phone"`
	MerchantID              int    `json:"merchant_id"`
	MerchantOrderID         string `json:"merchant_order_id"`
	MerchantRequestID       string `json:"merchant_request_id"`
	OrderID                 string `json:"order_id"`
	OriginatorConversationID string `json:"originator_conversation_id"`
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

type WithdrawalWebhookHandler struct {
	withdrawalRepo *repository.WithdrawalRepository
	walletRepo     *repository.WalletRepository
}

func NewWithdrawalWebhookHandler(
	withdrawalRepo *repository.WithdrawalRepository,
	walletRepo *repository.WalletRepository,
) *WithdrawalWebhookHandler {
	return &WithdrawalWebhookHandler{
		withdrawalRepo: withdrawalRepo,
		walletRepo:     walletRepo,
	}
}

// Handle processes the B2C callback. On COMPLETED: mark withdrawal done. On failure: refund withdrawable.
func (h *WithdrawalWebhookHandler) Handle(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("[Withdrawal callback] ReadBody error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	log.Printf("[Withdrawal callback] raw body: %s", string(body))
	var payload B2CCallback
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("[Withdrawal callback] json unmarshal error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	orderID := payload.MerchantOrderID
	if orderID == "" {
		orderID = payload.OrderID
	}
	if orderID == "" {
		orderID = payload.ReferenceOrderID
	}
	if orderID == "" {
		log.Printf("[Withdrawal callback] no order_id in payload")
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}
	w, err := h.withdrawalRepo.GetByOrderID(orderID)
	if err != nil || w == nil {
		log.Printf("[Withdrawal callback] withdrawal not found for order_id=%s", orderID)
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}
	if w.Status != "PENDING" {
		log.Printf("[Withdrawal callback] withdrawal %d already %s for order_id=%s", w.ID, w.Status, orderID)
		c.JSON(http.StatusOK, gin.H{"received": true})
		return
	}
	if payload.Status == "COMPLETED" {
		now := time.Now()
		w.Status = "COMPLETED"
		w.CompletedAt = &now
		if err := h.withdrawalRepo.Update(w); err != nil {
			log.Printf("[Withdrawal callback] update failed: %v", err)
		}
		log.Printf("[Withdrawal callback] withdrawal %d COMPLETED for order_id=%s", w.ID, orderID)
	} else {
		w.Status = "FAILED"
		if err := h.withdrawalRepo.Update(w); err != nil {
			log.Printf("[Withdrawal callback] update failed: %v", err)
		}
		_ = h.walletRepo.CreditWithdrawable(w.UserID, w.AmountCents)
		log.Printf("[Withdrawal callback] withdrawal %d FAILED, refunded %d cents to user %d", w.ID, w.AmountCents, w.UserID)
	}
	c.JSON(http.StatusOK, gin.H{"received": true})
}
