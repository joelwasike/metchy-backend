package repository

import (
	"lusty/internal/models"

	"gorm.io/gorm"
)

type CompanionRepository struct {
	db *gorm.DB
}

func NewCompanionRepository(db *gorm.DB) *CompanionRepository {
	return &CompanionRepository{db: db}
}

func (r *CompanionRepository) Create(p *models.CompanionProfile) error {
	return r.db.Create(p).Error
}

func (r *CompanionRepository) GetByID(id uint) (*models.CompanionProfile, error) {
	var p models.CompanionProfile
	err := r.db.Preload("Media").Preload("Pricing").First(&p, id).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *CompanionRepository) GetByUserID(userID uint) (*models.CompanionProfile, error) {
	var p models.CompanionProfile
	err := r.db.Preload("Media").Preload("Pricing").Where("user_id = ?", userID).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *CompanionRepository) Update(p *models.CompanionProfile) error {
	return r.db.Save(p).Error
}

func (r *CompanionRepository) AddMedia(m *models.CompanionMedia) error {
	return r.db.Create(m).Error
}

func (r *CompanionRepository) UpdateMedia(m *models.CompanionMedia) error {
	return r.db.Save(m).Error
}

func (r *CompanionRepository) DeleteMedia(id uint) error {
	return r.db.Delete(&models.CompanionMedia{}, id).Error
}

func (r *CompanionRepository) UpsertPricing(p *models.CompanionPricing) error {
	return r.db.Save(p).Error
}

func (r *CompanionRepository) GetPricingByID(id, companionID uint) (*models.CompanionPricing, error) {
	var p models.CompanionPricing
	err := r.db.Where("id = ? AND companion_id = ?", id, companionID).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetPricingByCompanionAndType returns existing pricing for a companion+type, or nil if not found.
func (r *CompanionRepository) GetPricingByCompanionAndType(companionID uint, ptype string) (*models.CompanionPricing, error) {
	var p models.CompanionPricing
	err := r.db.Where("companion_id = ? AND type = ?", companionID, ptype).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *CompanionRepository) DeletePricing(id, companionID uint) error {
	return r.db.Where("id = ? AND companion_id = ?", id, companionID).Delete(&models.CompanionPricing{}).Error
}

func (r *CompanionRepository) GetActiveBoost(companionID uint) (*models.CompanionBoost, error) {
	var b models.CompanionBoost
	err := r.db.Where("companion_id = ? AND is_active = ? AND end_at > NOW()", companionID, true).First(&b).Error
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *CompanionRepository) CreateBoost(b *models.CompanionBoost) error {
	return r.db.Create(b).Error
}
