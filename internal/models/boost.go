package models

import (
	"time"

	"gorm.io/gorm"
)

type CompanionBoost struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	CompanionID uint           `gorm:"not null;index" json:"companion_id"`
	BoostType   string         `gorm:"size:30;not null" json:"boost_type"` // e.g. 1h, 24h
	StartAt     time.Time      `gorm:"not null;index" json:"start_at"`
	EndAt       time.Time      `gorm:"not null;index" json:"end_at"`
	IsActive    bool           `gorm:"default:true;index" json:"is_active"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	Companion CompanionProfile `gorm:"foreignKey:CompanionID" json:"-"`
}

func (CompanionBoost) TableName() string {
	return "companion_boosts"
}
