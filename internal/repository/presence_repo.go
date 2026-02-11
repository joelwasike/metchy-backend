package repository

import (
	"lusty/internal/models"

	"gorm.io/gorm"
)

type PresenceRepository struct {
	db *gorm.DB
}

func NewPresenceRepository(db *gorm.DB) *PresenceRepository {
	return &PresenceRepository{db: db}
}

func (r *PresenceRepository) Upsert(p *models.UserPresence) error {
	return r.db.Save(p).Error
}

func (r *PresenceRepository) GetByUserID(userID uint) (*models.UserPresence, error) {
	var p models.UserPresence
	err := r.db.Where("user_id = ?", userID).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}
