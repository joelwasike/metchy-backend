package models

import (
	"time"

	"gorm.io/gorm"
)

type Block struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	BlockerID  uint           `gorm:"not null;index:idx_block_pair,unique" json:"blocker_id"`
	BlockedID  uint           `gorm:"not null;index:idx_block_pair,unique" json:"blocked_id"`
	CreatedAt  time.Time      `json:"created_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	Blocker User `gorm:"foreignKey:BlockerID" json:"-"`
	Blocked User `gorm:"foreignKey:BlockedID" json:"-"`
}

func (Block) TableName() string {
	return "blocks"
}

type Report struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	ReporterID uint           `gorm:"not null;index" json:"reporter_id"`
	ReportedID uint           `gorm:"not null;index" json:"reported_id"`
	Reason     string         `gorm:"size:50" json:"reason"`
	Details    string         `gorm:"type:text" json:"details"`
	Status     string         `gorm:"size:20;default:'PENDING';index" json:"status"` // PENDING, REVIEWED, RESOLVED
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	Reporter User `gorm:"foreignKey:ReporterID" json:"-"`
	Reported User `gorm:"foreignKey:ReportedID" json:"-"`
}

func (Report) TableName() string {
	return "reports"
}

type AuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     *uint     `gorm:"index" json:"user_id"`
	Action     string    `gorm:"size:100;not null;index" json:"action"`
	Resource   string    `gorm:"size:100;index" json:"resource"`
	ResourceID string    `gorm:"size:100;index" json:"resource_id"`
	IP         string    `gorm:"size:45" json:"ip"`
	UserAgent  string    `gorm:"size:512" json:"user_agent"`
	Metadata   string    `gorm:"type:text" json:"metadata"`
	CreatedAt  time.Time `json:"created_at"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}
