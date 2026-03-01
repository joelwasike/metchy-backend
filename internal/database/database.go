package database

import (
	"log"

	"lusty/config"
	"lusty/internal/domain"
	"lusty/internal/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewDB(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error), // Only log errors, not every SQL query
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	return db, nil
}

// AutoMigrate runs Gorm auto-migration for all models.
func AutoMigrate(db *gorm.DB) error {
	// Fix existing users with empty username before unique index is created.
	// Assign user_{id} to avoid duplicate key on idx_users_username.
	_ = db.Exec("UPDATE users SET username = CONCAT('user_', id) WHERE username = '' OR username IS NULL")
	return db.AutoMigrate(
		&models.User{},
		&models.Wallet{},
		&models.WalletTransaction{},
		&models.UserLocation{},
		&models.UserPresence{},
		&models.CompanionProfile{},
		&models.CompanionMedia{},
		&models.CompanionPricing{},
		&models.CompanionBoost{},
		&models.Favorite{},
		&models.Payment{},
		&models.InteractionRequest{},
		&models.ChatSession{},
		&models.ChatMessage{},
		&models.Notification{},
		&models.Block{},
		&models.Report{},
		&models.AuditLog{},
		&models.Withdrawal{},
		&models.ReferralCode{},
		&models.Referral{},
		&models.SystemSetting{},
	)
}

// SeedAdmin creates the default admin account if it doesn't exist.
func SeedAdmin(db *gorm.DB) {
	var count int64
	db.Model(&models.User{}).Where("role = ?", domain.RoleAdmin).Count(&count)
	if count > 0 {
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("admin@metchi2024"), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[seed] failed to hash admin password: %v", err)
		return
	}
	admin := &models.User{
		Email:        "admin@metchi.com",
		Username:     "admin",
		PasswordHash: string(hash),
		Role:         domain.RoleAdmin,
	}
	if err := db.Create(admin).Error; err != nil {
		log.Printf("[seed] failed to create admin user: %v", err)
		return
	}
	log.Printf("[seed] admin account created: admin@metchi.com")
}
