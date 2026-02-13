package handler

import (
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
	resp := gin.H{"balance_cents": w.BalanceCents, "currency": w.Currency}
	if w.WithdrawableCents != 0 {
		resp["withdrawable_cents"] = w.WithdrawableCents
	}
	c.JSON(http.StatusOK, resp)
}
