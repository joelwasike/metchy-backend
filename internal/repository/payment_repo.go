package repository

import (
	"lusty/internal/models"

	"gorm.io/gorm"
)

type PaymentRepository struct {
	db *gorm.DB
}

func NewPaymentRepository(db *gorm.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) Create(p *models.Payment) error {
	return r.db.Create(p).Error
}

func (r *PaymentRepository) GetByID(id uint) (*models.Payment, error) {
	var p models.Payment
	err := r.db.First(&p, id).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PaymentRepository) GetByProviderRef(ref string) (*models.Payment, error) {
	var p models.Payment
	err := r.db.Where("provider_ref = ?", ref).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PaymentRepository) GetByIdempotencyKey(key string) (*models.Payment, error) {
	var p models.Payment
	err := r.db.Where("idempotency_key = ?", key).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PaymentRepository) Update(p *models.Payment) error {
	return r.db.Save(p).Error
}
