package repository

import (
	"errors"
	"lusty/internal/models"

	"gorm.io/gorm"
)

var ErrInsufficientBalance = errors.New("insufficient wallet balance")

type WalletRepository struct {
	db *gorm.DB
}

func NewWalletRepository(db *gorm.DB) *WalletRepository {
	return &WalletRepository{db: db}
}

func (r *WalletRepository) GetByUserID(userID uint) (*models.Wallet, error) {
	var w models.Wallet
	err := r.db.Where("user_id = ?", userID).First(&w).Error
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *WalletRepository) GetOrCreate(userID uint) (*models.Wallet, error) {
	w, err := r.GetByUserID(userID)
	if err == nil {
		return w, nil
	}
	w = &models.Wallet{UserID: userID, BalanceCents: 0, WithdrawableCents: 0, Currency: "KES"}
	if err := r.db.Create(w).Error; err != nil {
		return nil, err
	}
	return w, nil
}

func (r *WalletRepository) Credit(userID uint, amountCents int64) error {
	w, err := r.GetOrCreate(userID)
	if err != nil {
		return err
	}
	w.BalanceCents += amountCents
	return r.db.Model(w).Update("balance_cents", w.BalanceCents).Error
}

func (r *WalletRepository) RecordTransaction(userID uint, amountCents int64, txType, reference string) error {
	return r.db.Create(&models.WalletTransaction{UserID: userID, AmountCents: amountCents, Type: txType, Reference: reference}).Error
}

func (r *WalletRepository) ListTransactionsByUserID(userID uint, limit, offset int) ([]models.WalletTransaction, error) {
	var list []models.WalletTransaction
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Offset(offset).Find(&list).Error
	return list, err
}

// DebitWithdrawable deducts from withdrawable (when initiating withdrawal).
func (r *WalletRepository) DebitWithdrawable(userID uint, amountCents int64) error {
	w, err := r.GetByUserID(userID)
	if err != nil {
		return err
	}
	if w.WithdrawableCents < amountCents {
		return ErrInsufficientBalance
	}
	w.WithdrawableCents -= amountCents
	if err := r.db.Model(w).Update("withdrawable_cents", w.WithdrawableCents).Error; err != nil {
		return err
	}
	return r.RecordTransaction(userID, -amountCents, "WITHDRAWAL", "")
}

// CreditWithdrawable adds to companion's withdrawable balance (when service is confirmed done).
func (r *WalletRepository) CreditWithdrawable(userID uint, amountCents int64) error {
	w, err := r.GetOrCreate(userID)
	if err != nil {
		return err
	}
	w.WithdrawableCents += amountCents
	return r.db.Model(w).Updates(map[string]interface{}{"withdrawable_cents": w.WithdrawableCents, "updated_at": w.UpdatedAt}).Error
}

func (r *WalletRepository) Debit(userID uint, amountCents int64) error {
	w, err := r.GetByUserID(userID)
	if err != nil {
		return err
	}
	if w.BalanceCents < amountCents {
		return ErrInsufficientBalance
	}
	w.BalanceCents -= amountCents
	if err := r.db.Model(w).Update("balance_cents", w.BalanceCents).Error; err != nil {
		return err
	}
	return r.RecordTransaction(userID, -amountCents, "DEBIT", "")
}
