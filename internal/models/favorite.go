package models

import (
	"time"

	"gorm.io/gorm"
)

type Favorite struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	ClientID    uint           `gorm:"not null;index:idx_fav_client_companion,unique" json:"client_id"`
	CompanionID uint           `gorm:"not null;index:idx_fav_client_companion,unique" json:"companion_id"`
	CreatedAt   time.Time      `json:"created_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	Client    User             `gorm:"foreignKey:ClientID" json:"-"`
	Companion CompanionProfile `gorm:"foreignKey:CompanionID" json:"-"`
}

func (Favorite) TableName() string {
	return "favorites"
}
