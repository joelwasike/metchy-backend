package models

import (
	"time"

	"lusty/internal/domain"

	"gorm.io/gorm"
)

type User struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	Username          string         `gorm:"uniqueIndex;size:64;not null;default:''" json:"username"`
	Email             string         `gorm:"uniqueIndex;size:255;not null" json:"email"`
	PasswordHash      string         `gorm:"size:255" json:"-"`
	Role              string         `gorm:"size:20;not null;index" json:"role"` // CLIENT | COMPANION
	DateOfBirth       *time.Time     `json:"date_of_birth"`
	EmailVerifiedAt   *time.Time     `json:"email_verified_at"`
	GoogleID          *string        `gorm:"uniqueIndex;size:255" json:"-"` // nil for email signups (avoids duplicate '' on unique index)
	AppleID           *string        `gorm:"uniqueIndex;size:255" json:"-"` // nil when not using Sign in with Apple
	AvatarURL         string         `gorm:"size:512" json:"avatar_url"`
	SearchRadiusKm    float64        `gorm:"default:10" json:"search_radius_km"` // Client: max search radius (default 10km)
	KYC               bool           `gorm:"default:false" json:"kyc"`
	FCMToken          string         `gorm:"size:512" json:"-"` // For push notifications
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	CompanionProfile *CompanionProfile `gorm:"foreignKey:UserID" json:"companion_profile,omitempty"`
	Location         *UserLocation     `gorm:"foreignKey:UserID" json:"location,omitempty"`
	Presence         *UserPresence     `gorm:"foreignKey:UserID" json:"presence,omitempty"`
}

func (u *User) IsCompanion() bool { return u.Role == domain.RoleCompanion }
func (u *User) IsClient() bool    { return u.Role == domain.RoleClient }

// Age returns age in years from DOB (caller must ensure DOB is set).
func (u *User) Age(t time.Time) int {
	if u.DateOfBirth == nil {
		return 0
	}
	age := t.Year() - u.DateOfBirth.Year()
	if t.YearDay() < u.DateOfBirth.YearDay() {
		age--
	}
	return age
}
