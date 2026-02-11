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
	w = &models.Wallet{UserID: userID, BalanceCents: 0, Currency: "KES"}
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

func (r *WalletRepository) Debit(userID uint, amountCents int64) error {
	w, err := r.GetByUserID(userID)
	if err != nil {
		return err
	}
	if w.BalanceCents < amountCents {
		return ErrInsufficientBalance
	}
	w.BalanceCents -= amountCents
	return r.db.Model(w).Update("balance_cents", w.BalanceCents).Error
}
