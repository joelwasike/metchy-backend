package models

import (
	"time"

	"gorm.io/gorm"
)

type Payment struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	UserID        uint           `gorm:"not null;index" json:"user_id"`
	AmountCents   int64          `gorm:"not null" json:"amount_cents"`
	Currency      string         `gorm:"size:3;default:'USD'" json:"currency"`
	Provider      string         `gorm:"size:50;not null" json:"provider"`
	ProviderRef   string         `gorm:"size:255;uniqueIndex" json:"provider_ref"`
	Status        string         `gorm:"size:20;not null;index" json:"status"` // PENDING, COMPLETED, FAILED, REFUNDED, EXPIRED
	IdempotencyKey string        `gorm:"size:255;uniqueIndex" json:"-"`
	Metadata      string         `gorm:"type:text" json:"metadata"` // JSON
	ExpiresAt     *time.Time     `json:"expires_at"`
	CompletedAt   *time.Time     `json:"completed_at"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

func (Payment) TableName() string {
	return "payments"
}
