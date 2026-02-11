package repository

import (
	"lusty/internal/models"

	"gorm.io/gorm"
)

type FavoriteRepository struct {
	db *gorm.DB
}

func NewFavoriteRepository(db *gorm.DB) *FavoriteRepository {
	return &FavoriteRepository{db: db}
}

func (r *FavoriteRepository) Add(clientID, companionID uint) error {
	return r.db.Create(&models.Favorite{ClientID: clientID, CompanionID: companionID}).Error
}

func (r *FavoriteRepository) Remove(clientID, companionID uint) error {
	return r.db.Where("client_id = ? AND companion_id = ?", clientID, companionID).Delete(&models.Favorite{}).Error
}

func (r *FavoriteRepository) IsFavorite(clientID, companionID uint) (bool, error) {
	var c int64
	err := r.db.Model(&models.Favorite{}).Where("client_id = ? AND companion_id = ?", clientID, companionID).Count(&c).Error
	return c > 0, err
}

func (r *FavoriteRepository) ListByClientID(clientID uint, limit, offset int) ([]models.Favorite, error) {
	var list []models.Favorite
	err := r.db.Where("client_id = ?", clientID).Preload("Companion").Limit(limit).Offset(offset).Find(&list).Error
	return list, err
}

// ListClientIDsByCompanionID returns user IDs of clients who favorited this companion (for "favorite online" notifications).
func (r *FavoriteRepository) ListClientIDsByCompanionID(companionID uint) ([]uint, error) {
	var ids []uint
	err := r.db.Model(&models.Favorite{}).Where("companion_id = ?", companionID).Pluck("client_id", &ids).Error
	return ids, err
}

func (r *FavoriteRepository) CountByCompanionID(companionID uint) (int64, error) {
	var c int64
	err := r.db.Model(&models.Favorite{}).Where("companion_id = ?", companionID).Count(&c).Error
	return c, err
}
