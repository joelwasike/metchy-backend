package repository

import (
	"lusty/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SettingRepository struct {
	db *gorm.DB
}

func NewSettingRepository(db *gorm.DB) *SettingRepository {
	return &SettingRepository{db: db}
}

func (r *SettingRepository) Get(key string) (string, error) {
	var s models.SystemSetting
	if err := r.db.Where("`key` = ?", key).First(&s).Error; err != nil {
		return "", err
	}
	return s.Value, nil
}

func (r *SettingRepository) Set(key, value string) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&models.SystemSetting{Key: key, Value: value}).Error
}

func (r *SettingRepository) GetAll() ([]models.SystemSetting, error) {
	var list []models.SystemSetting
	err := r.db.Order("`key` ASC").Find(&list).Error
	return list, err
}

// SeedDefaults inserts default settings if they don't already exist.
func (r *SettingRepository) SeedDefaults(defaults map[string]string) error {
	for k, v := range defaults {
		var count int64
		r.db.Model(&models.SystemSetting{}).Where("`key` = ?", k).Count(&count)
		if count == 0 {
			if err := r.db.Create(&models.SystemSetting{Key: k, Value: v}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
