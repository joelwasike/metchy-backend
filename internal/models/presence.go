package models

import (
	"time"

	"gorm.io/gorm"
)

type UserPresence struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	UserID     uint           `gorm:"uniqueIndex;not null" json:"user_id"`
	Status     string         `gorm:"size:20;not null;index" json:"status"` // ONLINE, OFFLINE, BUSY, IN_SESSION
	IsOnline   bool           `gorm:"default:false;index" json:"is_online"`
	LastSeenAt time.Time      `gorm:"not null;index" json:"last_seen_at"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

func (UserPresence) TableName() string {
	return "user_presence"
}
