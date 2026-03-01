package models

import (
	"time"

	"gorm.io/gorm"
)

// SystemSetting stores admin-configurable key/value settings.
type SystemSetting struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Key       string         `gorm:"uniqueIndex;size:100;not null" json:"key"`
	Value     string         `gorm:"size:255;not null" json:"value"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (SystemSetting) TableName() string { return "system_settings" }
