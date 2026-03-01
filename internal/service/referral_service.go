package service

import (
	"fmt"
	"log"
	"strconv"

	"lusty/internal/domain"
	"lusty/internal/models"
	"lusty/internal/repository"
)

// ReferralService handles referral code processing and bonus credits.
type ReferralService struct {
	referralRepo *repository.ReferralRepository
	walletRepo   *repository.WalletRepository
	settingRepo  *repository.SettingRepository
}

func NewReferralService(
	referralRepo *repository.ReferralRepository,
	walletRepo *repository.WalletRepository,
	settingRepo *repository.SettingRepository,
) *ReferralService {
	return &ReferralService{
		referralRepo: referralRepo,
		walletRepo:   walletRepo,
		settingRepo:  settingRepo,
	}
}

// ProcessReferralCode creates a referral record and credits bonus wallets for COMPANION signups.
// referralCode: the code submitted by the new user.
// newUser: the newly created user.
func (s *ReferralService) ProcessReferralCode(referralCode string, newUser *models.User) {
	if referralCode == "" || s.referralRepo == nil {
		return
	}
	rc, err := s.referralRepo.GetByCode(referralCode)
	if err != nil || rc == nil || rc.UserID == newUser.ID {
		return
	}

	// Create referral record
	if err := s.referralRepo.CreateReferral(&models.Referral{
		ReferrerID:     rc.UserID,
		ReferredUserID: newUser.ID,
	}); err != nil {
		log.Printf("[referral] failed to create referral: %v", err)
		return
	}

	// Only credit bonus for COMPANION signups
	if newUser.Role != domain.RoleCompanion {
		return
	}

	referrerBonus := s.getSettingInt(domain.SettingReferralBonusReferrer, 10000) // default KES 100 = 10000 cents
	referredBonus := s.getSettingInt(domain.SettingReferralBonusReferred, 20000) // default KES 200 = 20000 cents

	// Credit referrer wallet
	if referrerBonus > 0 {
		if err := s.walletRepo.Credit(rc.UserID, int64(referrerBonus)); err != nil {
			log.Printf("[referral] failed to credit referrer %d: %v", rc.UserID, err)
		}
		if err := s.walletRepo.CreditWithdrawable(rc.UserID, int64(referrerBonus)); err != nil {
			log.Printf("[referral] failed to credit referrer withdrawable %d: %v", rc.UserID, err)
		}
		_ = s.walletRepo.RecordTransaction(rc.UserID, int64(referrerBonus), domain.WalletTxTypeReferralBonus,
			fmt.Sprintf("referral_bonus_for_user_%d", newUser.ID))
	}

	// Credit new companion wallet
	if referredBonus > 0 {
		if err := s.walletRepo.Credit(newUser.ID, int64(referredBonus)); err != nil {
			log.Printf("[referral] failed to credit referred %d: %v", newUser.ID, err)
		}
		if err := s.walletRepo.CreditWithdrawable(newUser.ID, int64(referredBonus)); err != nil {
			log.Printf("[referral] failed to credit referred withdrawable %d: %v", newUser.ID, err)
		}
		_ = s.walletRepo.RecordTransaction(newUser.ID, int64(referredBonus), domain.WalletTxTypeReferralBonus,
			fmt.Sprintf("referral_signup_bonus_from_user_%d", rc.UserID))
	}
}

func (s *ReferralService) getSettingInt(key string, fallback int) int {
	val, err := s.settingRepo.Get(key)
	if err != nil || val == "" {
		return fallback
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return n
}
