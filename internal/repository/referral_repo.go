package repository

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"lusty/internal/models"

	"gorm.io/gorm"
)

type ReferralRepository struct {
	db *gorm.DB
}

func NewReferralRepository(db *gorm.DB) *ReferralRepository {
	return &ReferralRepository{db: db}
}

// generateCode returns an 8-character uppercase hex referral code.
func generateReferralCode() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil // 8 hex chars, e.g. "a3f2c1b0"
}

// GetOrCreateCode returns the existing referral code for a user, or creates a new unique one.
func (r *ReferralRepository) GetOrCreateCode(userID uint) (*models.ReferralCode, error) {
	var rc models.ReferralCode
	if err := r.db.Where("user_id = ?", userID).First(&rc).Error; err == nil {
		return &rc, nil
	}
	for i := 0; i < 10; i++ {
		code, err := generateReferralCode()
		if err != nil {
			return nil, err
		}
		rc = models.ReferralCode{UserID: userID, Code: code, IsActive: true}
		if err := r.db.Create(&rc).Error; err == nil {
			return &rc, nil
		}
		// Collision: retry with new code
	}
	return nil, fmt.Errorf("failed to generate a unique referral code after retries")
}

// GetByCode returns an active ReferralCode record matching the given code string.
func (r *ReferralRepository) GetByCode(code string) (*models.ReferralCode, error) {
	var rc models.ReferralCode
	err := r.db.Where("code = ? AND is_active = ?", code, true).First(&rc).Error
	if err != nil {
		return nil, err
	}
	return &rc, nil
}

// CreateReferral persists a new referral relationship.
func (r *ReferralRepository) CreateReferral(referral *models.Referral) error {
	return r.db.Create(referral).Error
}

// GetReferralByReferredUserID returns the Referral record for a user that was referred by someone.
// Returns nil, error if the user was not referred.
func (r *ReferralRepository) GetReferralByReferredUserID(userID uint) (*models.Referral, error) {
	var ref models.Referral
	err := r.db.Where("referred_user_id = ?", userID).First(&ref).Error
	if err != nil {
		return nil, err
	}
	return &ref, nil
}

// IncrementCompletedCount atomically increments the transaction count for a referral.
func (r *ReferralRepository) IncrementCompletedCount(referralID uint) error {
	return r.db.Model(&models.Referral{}).
		Where("id = ?", referralID).
		UpdateColumn("completed_count", gorm.Expr("completed_count + 1")).Error
}

// ListByReferrerID returns all referrals created by the given referrer, with referred user preloaded.
func (r *ReferralRepository) ListByReferrerID(referrerID uint, limit, offset int) ([]models.Referral, error) {
	var list []models.Referral
	err := r.db.Where("referrer_id = ?", referrerID).
		Preload("ReferredUser").
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&list).Error
	return list, err
}
