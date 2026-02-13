package repository

import (
	"lusty/internal/models"

	"gorm.io/gorm"
)

type WithdrawalRepository struct {
	db *gorm.DB
}

func NewWithdrawalRepository(db *gorm.DB) *WithdrawalRepository {
	return &WithdrawalRepository{db: db}
}

func (r *WithdrawalRepository) Create(w *models.Withdrawal) error {
	return r.db.Create(w).Error
}

func (r *WithdrawalRepository) GetByOrderID(orderID string) (*models.Withdrawal, error) {
	var w models.Withdrawal
	err := r.db.Where("order_id = ?", orderID).First(&w).Error
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *WithdrawalRepository) Update(w *models.Withdrawal) error {
	return r.db.Save(w).Error
}
