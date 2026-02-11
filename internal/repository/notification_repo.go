package repository

import (
	"lusty/internal/models"

	"gorm.io/gorm"
)

type NotificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) Create(n *models.Notification) error {
	return r.db.Create(n).Error
}

func (r *NotificationRepository) ListByUserID(userID uint, limit, offset int) ([]models.Notification, error) {
	var list []models.Notification
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Offset(offset).Find(&list).Error
	return list, err
}

func (r *NotificationRepository) MarkRead(id, userID uint) error {
	return r.db.Model(&models.Notification{}).Where("id = ? AND user_id = ?", id, userID).Update("read_at", gorm.Expr("NOW()")).Error
}
