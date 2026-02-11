package models

import (
	"time"

	"gorm.io/gorm"
)

type Notification struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `gorm:"not null;index" json:"user_id"`
	Type      string         `gorm:"size:50;not null;index" json:"type"`
	Title     string         `gorm:"size:255" json:"title"`
	Body      string         `gorm:"type:text" json:"body"`
	Data      string         `gorm:"type:text" json:"data"` // JSON payload
	ReadAt    *time.Time     `json:"read_at"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

func (Notification) TableName() string {
	return "notifications"
}
