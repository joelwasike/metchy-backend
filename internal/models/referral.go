package models

import (
	"time"

	"gorm.io/gorm"
)

// ReferralCode is a unique invite code belonging to a user.
// Each user has at most one referral code.
type ReferralCode struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `gorm:"uniqueIndex;not null" json:"user_id"`
	Code      string         `gorm:"uniqueIndex;size:20;not null" json:"code"`
	IsActive  bool           `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

func (ReferralCode) TableName() string { return "referral_codes" }

// Referral tracks the relationship between a referrer and a referred user.
// A user can only be referred once. Commission is paid for the first 2 qualifying transactions.
// For a referred COMPANION (female service provider): commission is earned when their service is confirmed done.
// For a referred CLIENT (male): commission is earned when they complete a payment/order.
type Referral struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	ReferrerID     uint           `gorm:"not null;index" json:"referrer_id"`
	ReferredUserID uint           `gorm:"uniqueIndex;not null" json:"referred_user_id"` // each user can only be referred once
	CompletedCount int            `gorm:"not null;default:0" json:"completed_count"`     // number of qualifying transactions (max 2)
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	Referrer     User `gorm:"foreignKey:ReferrerID" json:"referrer,omitempty"`
	ReferredUser User `gorm:"foreignKey:ReferredUserID" json:"referred_user,omitempty"`
}

func (Referral) TableName() string { return "referrals" }
