package handler

import (
	"fmt"
	"net/http"

	"lusty/internal/middleware"
	"lusty/internal/repository"

	"github.com/gin-gonic/gin"
)

type WalletHandler struct {
	walletRepo *repository.WalletRepository
}

func NewWalletHandler(walletRepo *repository.WalletRepository) *WalletHandler {
	return &WalletHandler{walletRepo: walletRepo}
}

// GetBalance returns the current user's wallet balance (CLIENT only - companions use earnings).
func (h *WalletHandler) GetBalance(c *gin.Context) {
	userID := middleware.GetUserID(c)
	w, err := h.walletRepo.GetOrCreate(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "wallet error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"balance_cents":       w.BalanceCents,
		"withdrawable_cents":  w.WithdrawableCents,
		"currency":            w.Currency,
	})
}

// GetTransactions returns the current user's wallet transaction history (companion earnings, withdrawals).
func (h *WalletHandler) GetTransactions(c *gin.Context) {
	userID := middleware.GetUserID(c)
	limit := 50
	if l := c.Query("limit"); l != "" {
		if n, err := parseInt(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	offset := 0
	if o := c.Query("offset"); o != "" {
		if n, err := parseInt(o); err == nil && n >= 0 {
			offset = n
		}
	}
	list, err := h.walletRepo.ListTransactionsByUserID(userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load transactions"})
		return
	}
	out := make([]gin.H, 0, len(list))
	for _, t := range list {
		out = append(out, gin.H{
			"id":            t.ID,
			"amount_cents":  t.AmountCents,
			"type":          t.Type,
			"reference":     t.Reference,
			"created_at":    t.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"transactions": out})
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
