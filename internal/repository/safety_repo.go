package repository

import (
	"lusty/internal/models"

	"gorm.io/gorm"
)

type BlockRepository struct {
	db *gorm.DB
}

func NewBlockRepository(db *gorm.DB) *BlockRepository {
	return &BlockRepository{db: db}
}

func (r *BlockRepository) Create(b *models.Block) error {
	return r.db.Create(b).Error
}

func (r *BlockRepository) Delete(blockerID, blockedID uint) error {
	return r.db.Where("blocker_id = ? AND blocked_id = ?", blockerID, blockedID).Delete(&models.Block{}).Error
}

func (r *BlockRepository) IsBlocked(blockerID, blockedID uint) (bool, error) {
	var c int64
	err := r.db.Model(&models.Block{}).Where("blocker_id = ? AND blocked_id = ?", blockerID, blockedID).Count(&c).Error
	return c > 0, err
}

type ReportRepository struct {
	db *gorm.DB
}

func NewReportRepository(db *gorm.DB) *ReportRepository {
	return &ReportRepository{db: db}
}

func (r *ReportRepository) Create(report *models.Report) error {
	return r.db.Create(report).Error
}

func (r *ReportRepository) ListPending(limit int) ([]models.Report, error) {
	var list []models.Report
	err := r.db.Where("status = ?", "PENDING").Limit(limit).Find(&list).Error
	return list, err
}

type AuditLogRepository struct {
	db *gorm.DB
}

func NewAuditLogRepository(db *gorm.DB) *AuditLogRepository {
	return &AuditLogRepository{db: db}
}

func (r *AuditLogRepository) Create(log *models.AuditLog) error {
	return r.db.Create(log).Error
}
