package models

import (
	"time"

	"gorm.io/gorm"
)

// WalletTransaction records credits/debits for wallet history (companion earnings, withdrawals, boost).
type WalletTransaction struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	UserID      uint           `gorm:"not null;index" json:"user_id"`
	AmountCents int64          `gorm:"not null" json:"amount_cents"` // positive = credit, negative = debit
	Type        string         `gorm:"size:30;not null;index" json:"type"` // EARNING, WITHDRAWAL, BOOST_PAYMENT
	Reference   string         `gorm:"size:128" json:"reference"`        // e.g. interaction_id, withdrawal_id
	CreatedAt   time.Time      `json:"created_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

func (WalletTransaction) TableName() string {
	return "wallet_transactions"
}
