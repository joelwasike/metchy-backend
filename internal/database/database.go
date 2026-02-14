package database

import (
	"lusty/config"
	"lusty/internal/models"

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
	)
}
