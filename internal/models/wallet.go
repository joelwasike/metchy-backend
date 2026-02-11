package models

import (
	"time"

	"gorm.io/gorm"
)

type Wallet struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	UserID       uint           `gorm:"uniqueIndex;not null" json:"user_id"`
	BalanceCents int64          `gorm:"not null;default:0" json:"balance_cents"`
	Currency     string         `gorm:"size:3;default:'KES'" json:"currency"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

func (Wallet) TableName() string {
	return "wallets"
}
