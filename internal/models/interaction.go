package models

import (
	"time"

	"lusty/internal/domain"

	"gorm.io/gorm"
)

type InteractionRequest struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	ClientID         uint           `gorm:"not null;index" json:"client_id"`
	CompanionID      uint           `gorm:"not null;index" json:"companion_id"`
	InteractionType  string         `gorm:"size:20;not null;index" json:"interaction_type"` // CHAT, VIDEO, BOOKING
	PaymentID        *uint          `gorm:"index" json:"payment_id"`
	Status           string         `gorm:"size:20;not null;index" json:"status"` // PENDING, PENDING_KYC, ACCEPTED, REJECTED, EXPIRED
	DurationMinutes  int            `json:"duration_minutes"`
	ExpiresAt          *time.Time     `json:"expires_at"`
	AcceptedAt         *time.Time     `json:"accepted_at"`
	RejectedAt         *time.Time     `json:"rejected_at"`
	ServiceCompletedAt *time.Time     `json:"service_completed_at"` // set when client confirms service done
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`

	Client    User            `gorm:"foreignKey:ClientID" json:"-"`
	Companion CompanionProfile `gorm:"foreignKey:CompanionID" json:"-"`
	Payment   *Payment        `gorm:"foreignKey:PaymentID" json:"payment,omitempty"`
}

func (InteractionRequest) TableName() string {
	return "interaction_requests"
}

func (r *InteractionRequest) IsAccepted() bool { return r.Status == domain.RequestStatusAccepted }

type ChatSession struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	InteractionID     uint           `gorm:"uniqueIndex;not null" json:"interaction_id"`
	StartedAt         time.Time      `json:"started_at"`
	EndsAt            time.Time      `gorm:"index" json:"ends_at"`
	EndedAt           *time.Time     `json:"ended_at"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`

	Interaction InteractionRequest `gorm:"foreignKey:InteractionID" json:"-"`
}

func (ChatSession) TableName() string {
	return "chat_sessions"
}

type ChatMessage struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	SessionID  uint           `gorm:"not null;index" json:"session_id"`
	SenderID   uint           `gorm:"not null;index" json:"sender_id"`
	Content    string         `gorm:"type:text" json:"content"`
	MediaURL   string         `gorm:"size:512" json:"media_url"`
	CreatedAt  time.Time      `json:"created_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	Session ChatSession `gorm:"foreignKey:SessionID" json:"-"`
	Sender  User        `gorm:"foreignKey:SenderID" json:"-"`
}

func (ChatMessage) TableName() string {
	return "chat_messages"
}
