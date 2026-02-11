package repository

import (
	"lusty/internal/models"

	"gorm.io/gorm"
)

type LocationRepository struct {
	db *gorm.DB
}

func NewLocationRepository(db *gorm.DB) *LocationRepository {
	return &LocationRepository{db: db}
}

func (r *LocationRepository) Upsert(loc *models.UserLocation) error {
	return r.db.Save(loc).Error
}

func (r *LocationRepository) GetByUserID(userID uint) (*models.UserLocation, error) {
	var loc models.UserLocation
	err := r.db.Where("user_id = ?", userID).First(&loc).Error
	if err != nil {
		return nil, err
	}
	return &loc, nil
}

func (r *LocationRepository) GetLocationByUserID(userID uint) (*models.UserLocation, error) {
	return r.GetByUserID(userID)
}
