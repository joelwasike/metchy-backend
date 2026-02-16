package repository

import (
	"encoding/json"
	"time"

	"lusty/internal/models"

	"gorm.io/gorm"
)

type InteractionRepository struct {
	db *gorm.DB
}

func NewInteractionRepository(db *gorm.DB) *InteractionRepository {
	return &InteractionRepository{db: db}
}

func (r *InteractionRepository) Create(req *models.InteractionRequest) error {
	return r.db.Create(req).Error
}

func (r *InteractionRepository) GetByID(id uint) (*models.InteractionRequest, error) {
	var req models.InteractionRequest
	err := r.db.Preload("Payment").Preload("Companion").First(&req, id).Error
	if err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *InteractionRepository) Update(req *models.InteractionRequest) error {
	return r.db.Save(req).Error
}

func (r *InteractionRepository) ListPendingForCompanion(companionID uint, limit int) ([]models.InteractionRequest, error) {
	var list []models.InteractionRequest
	err := r.db.Where("companion_id = ? AND status = ?", companionID, "PENDING").Preload("Client").Limit(limit).Find(&list).Error
	return list, err
}

// CountPendingByCompanionID returns the number of PENDING requests for the companion (for badge).
func (r *InteractionRepository) CountPendingByCompanionID(companionID uint) (int64, error) {
	var c int64
	err := r.db.Model(&models.InteractionRequest{}).Where("companion_id = ? AND status = ?", companionID, "PENDING").Count(&c).Error
	return c, err
}

// RejectOtherPendingByCompanionID sets all other PENDING requests for this companion to REJECTED (except exceptInteractionID).
// Call after companion accepts one request so others are auto-removed.
func (r *InteractionRepository) RejectOtherPendingByCompanionID(companionID, exceptInteractionID uint) error {
	return r.db.Model(&models.InteractionRequest{}).
		Where("companion_id = ? AND status = ? AND id != ?", companionID, "PENDING", exceptInteractionID).
		Updates(map[string]interface{}{"status": "REJECTED", "rejected_at": time.Now()}).Error
}

// ListPendingKycByClientID returns interactions with status PENDING_KYC for the client (payment done, KYC not complete yet).
func (r *InteractionRepository) ListPendingKycByClientID(clientID uint, limit int) ([]models.InteractionRequest, error) {
	var list []models.InteractionRequest
	err := r.db.Where("client_id = ? AND status = ?", clientID, "PENDING_KYC").Preload("Payment").Preload("Companion").Limit(limit).Find(&list).Error
	return list, err
}

func (r *InteractionRepository) ListByClientID(clientID uint, limit, offset int) ([]models.InteractionRequest, error) {
	var list []models.InteractionRequest
	err := r.db.Where("client_id = ?", clientID).Preload("Companion").Limit(limit).Offset(offset).Order("created_at DESC").Find(&list).Error
	return list, err
}

func (r *InteractionRepository) ListByCompanionID(companionID uint, limit, offset int) ([]models.InteractionRequest, error) {
	var list []models.InteractionRequest
	err := r.db.Where("companion_id = ?", companionID).Preload("Client").Preload("Payment").Limit(limit).Offset(offset).Order("created_at DESC").Find(&list).Error
	return list, err
}

// GetLatestByClientAndCompanion returns the most recent interaction between client (user id) and companion (profile id).
func (r *InteractionRepository) GetLatestByClientAndCompanion(clientUserID, companionProfileID uint) (*models.InteractionRequest, error) {
	var ir models.InteractionRequest
	err := r.db.Where("client_id = ? AND companion_id = ?", clientUserID, companionProfileID).
		Preload("Payment").Preload("Companion").
		Order("created_at DESC").First(&ir).Error
	if err != nil {
		return nil, err
	}
	return &ir, nil
}

func (r *InteractionRepository) GetByPaymentID(paymentID uint) (*models.InteractionRequest, error) {
	var ir models.InteractionRequest
	err := r.db.Where("payment_id = ?", paymentID).Preload("Companion").Preload("Client").Preload("Payment").First(&ir).Error
	if err != nil {
		return nil, err
	}
	return &ir, nil
}

// ChatSession
func (r *InteractionRepository) CreateChatSession(s *models.ChatSession) error {
	return r.db.Create(s).Error
}

func (r *InteractionRepository) GetChatSessionByInteractionID(interactionID uint) (*models.ChatSession, error) {
	var s models.ChatSession
	err := r.db.Where("interaction_id = ?", interactionID).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *InteractionRepository) UpdateChatSession(s *models.ChatSession) error {
	return r.db.Save(s).Error
}

// ChatMessage
func (r *InteractionRepository) CreateMessage(m *models.ChatMessage) error {
	return r.db.Create(m).Error
}

func (r *InteractionRepository) GetMessagesBySessionID(sessionID uint, limit, offset int) ([]models.ChatMessage, error) {
	var list []models.ChatMessage
	err := r.db.Where("session_id = ?", sessionID).Order("created_at ASC").Limit(limit).Offset(offset).Find(&list).Error
	return list, err
}

// DeleteMessagesBySessionID soft-deletes all messages in a session.
func (r *InteractionRepository) DeleteMessagesBySessionID(sessionID uint) error {
	return r.db.Where("session_id = ?", sessionID).Delete(&models.ChatMessage{}).Error
}

// GetEarningsByCompanionID returns total earnings (amount_cents) from completed payments for accepted interactions.
func (r *InteractionRepository) GetEarningsByCompanionID(companionID uint) (int64, error) {
	var sum int64
	err := r.db.Table("interaction_requests ir").
		Select("COALESCE(SUM(p.amount_cents), 0)").
		Joins("INNER JOIN payments p ON ir.payment_id = p.id AND p.deleted_at IS NULL").
		Where("ir.companion_id = ? AND ir.status = ? AND p.status = ?", companionID, "ACCEPTED", "COMPLETED").
		Scan(&sum).Error
	return sum, err
}

// CountActiveSessionsByCompanionID returns count of chat sessions that are active (not ended, ends_at > now).
func (r *InteractionRepository) CountActiveSessionsByCompanionID(companionID uint) (int64, error) {
	var c int64
	err := r.db.Table("chat_sessions cs").
		Joins("INNER JOIN interaction_requests ir ON cs.interaction_id = ir.id").
		Where("ir.companion_id = ? AND cs.deleted_at IS NULL AND cs.ended_at IS NULL AND cs.ends_at > NOW()", companionID).
		Count(&c).Error
	return c, err
}

// ActiveSessionRow is a row for companion active sessions list.
type ActiveSessionRow struct {
	InteractionID   uint
	ClientName      string
	ServiceType     string
	DurationMinutes int
	StartedAt       time.Time
	EndsAt          time.Time
}

// ListActiveSessionsByCompanionID returns active chat sessions for the companion with client name, service type, duration.
func (r *InteractionRepository) ListActiveSessionsByCompanionID(companionID uint, limit int) ([]ActiveSessionRow, error) {
	var list []struct {
		ID               uint
		ClientID         uint
		InteractionType  string
		DurationMinutes  int
		StartedAt        time.Time
		EndsAt           time.Time
		Username         string
		Email            string
		PaymentMetadata  string
	}
	err := r.db.Table("interaction_requests ir").
		Select("ir.id, ir.client_id, ir.interaction_type, ir.duration_minutes, cs.started_at, cs.ends_at, u.username, u.email, p.metadata as payment_metadata").
		Joins("INNER JOIN chat_sessions cs ON cs.interaction_id = ir.id AND cs.deleted_at IS NULL AND cs.ended_at IS NULL AND cs.ends_at > NOW()").
		Joins("INNER JOIN users u ON u.id = ir.client_id").
		Joins("LEFT JOIN payments p ON p.id = ir.payment_id AND p.deleted_at IS NULL").
		Where("ir.companion_id = ? AND ir.status = ? AND ir.deleted_at IS NULL", companionID, "ACCEPTED").
		Order("cs.started_at DESC").
		Limit(limit).
		Scan(&list).Error
	if err != nil {
		return nil, err
	}
	out := make([]ActiveSessionRow, 0, len(list))
	for _, row := range list {
		clientName := row.Username
		if clientName == "" {
			clientName = row.Email
		}
		if clientName == "" {
			clientName = "Client"
		}
		svcType := row.InteractionType
		if row.PaymentMetadata != "" {
			var meta struct {
				ServiceType string `json:"service_type"`
			}
			_ = json.Unmarshal([]byte(row.PaymentMetadata), &meta)
			if meta.ServiceType != "" {
				svcType = meta.ServiceType
			}
		}
		out = append(out, ActiveSessionRow{
			InteractionID:   row.ID,
			ClientName:      clientName,
			ServiceType:     svcType,
			DurationMinutes: row.DurationMinutes,
			StartedAt:       row.StartedAt,
			EndsAt:          row.EndsAt,
		})
	}
	return out, nil
}

// ClientHasActiveSessionWithOtherCompanion returns true if the client has an active chat session
// with any companion other than excludeCompanionID. Used to disable "Interested" when client
// is already in an active chat with another companion.
func (r *InteractionRepository) ClientHasActiveSessionWithOtherCompanion(clientID, excludeCompanionID uint) (bool, error) {
	var c int64
	err := r.db.Table("chat_sessions cs").
		Joins("INNER JOIN interaction_requests ir ON cs.interaction_id = ir.id").
		Where("ir.client_id = ? AND ir.companion_id != ? AND ir.status = ? AND cs.deleted_at IS NULL AND cs.ended_at IS NULL AND cs.ends_at > NOW()",
			clientID, excludeCompanionID, "ACCEPTED").
		Limit(1).
		Count(&c).Error
	return c > 0, err
}
