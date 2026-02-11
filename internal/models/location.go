package models

import (
	"time"

	"gorm.io/gorm"
)

// UserLocation stores lat/lng with optional spatial index.
// Using separate lat/lng columns for portability and Haversine queries.
type UserLocation struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	UserID            uint           `gorm:"uniqueIndex;not null" json:"user_id"`
	Latitude          float64        `gorm:"type:decimal(10,8);not null;index:idx_location_lat_lng" json:"-"`
	Longitude         float64        `gorm:"type:decimal(11,8);not null;index:idx_location_lat_lng" json:"-"`
	AccuracyMeters   float64        `gorm:"type:decimal(8,2)" json:"accuracy_meters"`
	IsLocationVisible bool          `gorm:"default:true" json:"is_location_visible"`
	LastUpdatedAt     time.Time     `gorm:"not null;index" json:"last_updated_at"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

// TableName allows custom table name.
func (UserLocation) TableName() string {
	return "user_locations"
}
