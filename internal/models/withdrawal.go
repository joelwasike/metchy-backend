package models

import (
	"time"

	"gorm.io/gorm"
)

type Withdrawal struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	UserID       uint           `gorm:"not null;index" json:"user_id"`
	OrderID      string         `gorm:"size:64;uniqueIndex;not null" json:"order_id"`
	AmountCents  int64          `gorm:"not null" json:"amount_cents"`
	PhoneNumber  string         `gorm:"size:20;not null" json:"phone_number"`
	Status       string         `gorm:"size:20;not null;index" json:"status"` // PENDING, COMPLETED, FAILED
	ProviderRef  string         `gorm:"size:128" json:"provider_ref"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	CompletedAt  *time.Time     `json:"completed_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

func (Withdrawal) TableName() string {
	return "withdrawals"
}
