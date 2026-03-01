package models

import (
	"time"

	"gorm.io/gorm"
)

type CompanionProfile struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	UserID            uint           `gorm:"uniqueIndex;not null" json:"user_id"`
	DisplayName       string         `gorm:"size:100;not null" json:"display_name"`
	Bio               string         `gorm:"type:text" json:"bio"`
	Interests         string         `gorm:"type:text" json:"interests"` // JSON or comma-separated
	Categories        string         `gorm:"type:text" json:"categories"` // e.g. "tall,slim,long_hair"
	Languages         string         `gorm:"size:255" json:"languages"`
	CityOrArea        string         `gorm:"size:100;index" json:"city_or_area"`
	AvailabilityStatus string        `gorm:"size:50" json:"availability_status"`
	MainProfileImageURL string      `gorm:"size:512" json:"main_profile_image_url"`
	IsActive          bool           `gorm:"default:true;index" json:"is_active"`
	AppearInSearch    bool           `gorm:"default:true;index" json:"appear_in_search"`
	AcceptNewRequests bool           `gorm:"default:true" json:"accept_new_requests"`
	IsAvailable       bool           `gorm:"default:true;index" json:"is_available"` // manual toggle; set false when companion accepts a request
	OnboardingCompletedAt *time.Time `json:"onboarding_completed_at"` // nil = needs onboarding
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`

	User     User           `gorm:"foreignKey:UserID" json:"-"`
	Media    []CompanionMedia `gorm:"foreignKey:CompanionID" json:"media,omitempty"`
	Pricing  []CompanionPricing `gorm:"foreignKey:CompanionID" json:"pricing,omitempty"`
}

func (CompanionProfile) TableName() string {
	return "companion_profiles"
}

type CompanionMedia struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	CompanionID  uint           `gorm:"not null;index" json:"companion_id"`
	MediaType    string         `gorm:"size:20;not null" json:"media_type"`   // IMAGE | VIDEO
	URL          string         `gorm:"size:512;not null" json:"url"`
	ThumbnailURL string         `gorm:"size:512" json:"thumbnail_url"`
	Visibility   string         `gorm:"size:20;default:'PUBLIC'" json:"visibility"` // PUBLIC | PRIVATE
	SortOrder    int            `gorm:"default:0" json:"sort_order"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	Companion CompanionProfile `gorm:"foreignKey:CompanionID" json:"-"`
}

func (CompanionMedia) TableName() string {
	return "companion_media"
}

type CompanionPricing struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	CompanionID   uint           `gorm:"not null;index" json:"companion_id"`
	Type          string         `gorm:"size:30;not null;index" json:"type"` // CHAT_ACCESS, VIDEO_PER_5MIN, BOOKING_FEE, or service: SEX, MASSAGE, etc.
	Unit          string         `gorm:"size:20" json:"unit"`                // per_service, per_hour, per_night (for service pricing)
	CustomName    string         `gorm:"size:100" json:"custom_name"`        // display name for CUSTOM type
	AmountCents   int64          `gorm:"not null" json:"amount_cents"`
	Currency      string         `gorm:"size:3;default:'USD'" json:"currency"`
	IsActive      bool           `gorm:"default:true" json:"is_active"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	Companion CompanionProfile `gorm:"foreignKey:CompanionID" json:"-"`
}

func (CompanionPricing) TableName() string {
	return "companion_pricing"
}
