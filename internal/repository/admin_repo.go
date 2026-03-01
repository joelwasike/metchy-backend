package repository

import (
	"time"

	"lusty/internal/models"

	"gorm.io/gorm"
)

type DashboardStats struct {
	TotalUsers        int64 `json:"total_users"`
	TotalCompanions   int64 `json:"total_companions"`
	TotalClients      int64 `json:"total_clients"`
	OnlineUsers       int64 `json:"online_users"`
	TotalRevenue      int64 `json:"total_revenue"`
	TotalTransactions int64 `json:"total_transactions"`
	PendingReports    int64 `json:"pending_reports"`
	TotalReferrals    int64 `json:"total_referrals"`
	TotalWithdrawals  int64 `json:"total_withdrawals"`
	TotalInteractions int64 `json:"total_interactions"`
	PlatformProfit    int64 `json:"platform_profit"`
}

type TimeSeriesPoint struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

type RevenuePoint struct {
	Date         string `json:"date"`
	AmountCents  int64  `json:"amount_cents"`
}

type AdminRepository struct {
	db *gorm.DB
}

func NewAdminRepository(db *gorm.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

func (r *AdminRepository) GetDashboardStats() (*DashboardStats, error) {
	var s DashboardStats
	r.db.Model(&models.User{}).Count(&s.TotalUsers)
	r.db.Model(&models.User{}).Where("role = ?", "COMPANION").Count(&s.TotalCompanions)
	r.db.Model(&models.User{}).Where("role = ?", "CLIENT").Count(&s.TotalClients)
	r.db.Model(&models.UserPresence{}).Where("is_online = ?", true).Count(&s.OnlineUsers)

	var rev struct{ Total int64 }
	r.db.Model(&models.Payment{}).Select("COALESCE(SUM(amount_cents), 0) as total").Where("status = ?", "COMPLETED").Scan(&rev)
	s.TotalRevenue = rev.Total

	r.db.Model(&models.WalletTransaction{}).Count(&s.TotalTransactions)
	r.db.Model(&models.Report{}).Where("status = ?", "PENDING").Count(&s.PendingReports)
	r.db.Model(&models.Referral{}).Count(&s.TotalReferrals)
	r.db.Model(&models.Withdrawal{}).Count(&s.TotalWithdrawals)
	r.db.Model(&models.InteractionRequest{}).Count(&s.TotalInteractions)

	var profit struct{ Total int64 }
	r.db.Model(&models.WalletTransaction{}).Select("COALESCE(SUM(amount_cents), 0) as total").Where("type = ?", "PLATFORM_FEE").Scan(&profit)
	s.PlatformProfit = profit.Total

	return &s, nil
}

// ListUsers returns users with search, role filter, and pagination.
func (r *AdminRepository) ListUsers(search, role string, page, limit int) ([]models.User, int64, error) {
	q := r.db.Model(&models.User{})
	if search != "" {
		q = q.Where("username LIKE ? OR email LIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if role != "" {
		q = q.Where("role = ?", role)
	}
	var total int64
	q.Count(&total)
	var users []models.User
	err := q.Preload("CompanionProfile").Preload("Presence").Order("created_at DESC").Limit(limit).Offset((page - 1) * limit).Find(&users).Error
	return users, total, err
}

// GetUserByID returns a user with all relations.
func (r *AdminRepository) GetUserByID(id uint) (*models.User, error) {
	var u models.User
	err := r.db.Preload("CompanionProfile").Preload("Location").Preload("Presence").First(&u, id).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// ListCompanions returns companion profiles with search and pagination.
func (r *AdminRepository) ListCompanions(search string, page, limit int) ([]models.CompanionProfile, int64, error) {
	q := r.db.Model(&models.CompanionProfile{})
	if search != "" {
		q = q.Where("display_name LIKE ? OR city_or_area LIKE ?", "%"+search+"%", "%"+search+"%")
	}
	var total int64
	q.Count(&total)
	var list []models.CompanionProfile
	err := q.Preload("User").Order("created_at DESC").Limit(limit).Offset((page - 1) * limit).Find(&list).Error
	return list, total, err
}

// ListTransactions returns wallet transactions with optional type filter.
func (r *AdminRepository) ListTransactions(txType string, page, limit int) ([]models.WalletTransaction, int64, error) {
	q := r.db.Model(&models.WalletTransaction{})
	if txType != "" {
		q = q.Where("type = ?", txType)
	}
	var total int64
	q.Count(&total)
	var list []models.WalletTransaction
	err := q.Preload("User").Order("created_at DESC").Limit(limit).Offset((page - 1) * limit).Find(&list).Error
	return list, total, err
}

// ListPayments returns payments with optional status filter.
func (r *AdminRepository) ListPayments(status string, page, limit int) ([]models.Payment, int64, error) {
	q := r.db.Model(&models.Payment{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	q.Count(&total)
	var list []models.Payment
	err := q.Preload("User").Order("created_at DESC").Limit(limit).Offset((page - 1) * limit).Find(&list).Error
	return list, total, err
}

// ListWithdrawals returns withdrawals with optional status filter.
func (r *AdminRepository) ListWithdrawals(status string, page, limit int) ([]models.Withdrawal, int64, error) {
	q := r.db.Model(&models.Withdrawal{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	q.Count(&total)
	var list []models.Withdrawal
	err := q.Preload("User").Order("created_at DESC").Limit(limit).Offset((page - 1) * limit).Find(&list).Error
	return list, total, err
}

// ListInteractions returns interaction requests with optional status filter.
func (r *AdminRepository) ListInteractions(status string, page, limit int) ([]models.InteractionRequest, int64, error) {
	q := r.db.Model(&models.InteractionRequest{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	q.Count(&total)
	var list []models.InteractionRequest
	err := q.Preload("Client").Order("created_at DESC").Limit(limit).Offset((page - 1) * limit).Find(&list).Error
	return list, total, err
}

// ListReports returns reports with optional status filter.
func (r *AdminRepository) ListReports(status string, page, limit int) ([]models.Report, int64, error) {
	q := r.db.Model(&models.Report{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	q.Count(&total)
	var list []models.Report
	err := q.Preload("Reporter").Preload("Reported").Order("created_at DESC").Limit(limit).Offset((page - 1) * limit).Find(&list).Error
	return list, total, err
}

// UpdateReportStatus updates a report's status.
func (r *AdminRepository) UpdateReportStatus(id uint, status string) error {
	return r.db.Model(&models.Report{}).Where("id = ?", id).Update("status", status).Error
}

// ListReferrals returns all referrals with preloaded users.
func (r *AdminRepository) ListReferrals(page, limit int) ([]models.Referral, int64, error) {
	var total int64
	r.db.Model(&models.Referral{}).Count(&total)
	var list []models.Referral
	err := r.db.Preload("Referrer").Preload("ReferredUser").Order("created_at DESC").Limit(limit).Offset((page - 1) * limit).Find(&list).Error
	return list, total, err
}

// ListOnlineUsers returns users with online presence.
func (r *AdminRepository) ListOnlineUsers(page, limit int) ([]models.UserPresence, int64, error) {
	q := r.db.Model(&models.UserPresence{}).Where("is_online = ?", true)
	var total int64
	q.Count(&total)
	var list []models.UserPresence
	err := r.db.Where("is_online = ?", true).Preload("User").Order("last_seen_at DESC").Limit(limit).Offset((page - 1) * limit).Find(&list).Error
	return list, total, err
}

// UserSignupsByDay returns daily signup counts for the last N days.
func (r *AdminRepository) UserSignupsByDay(days int) ([]TimeSeriesPoint, error) {
	since := time.Now().AddDate(0, 0, -days)
	var points []TimeSeriesPoint
	err := r.db.Model(&models.User{}).
		Select("DATE(created_at) as date, COUNT(*) as count").
		Where("created_at >= ?", since).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&points).Error
	return points, err
}

// RevenueByDay returns daily completed payment revenue for the last N days.
func (r *AdminRepository) RevenueByDay(days int) ([]RevenuePoint, error) {
	since := time.Now().AddDate(0, 0, -days)
	var points []RevenuePoint
	err := r.db.Model(&models.Payment{}).
		Select("DATE(completed_at) as date, COALESCE(SUM(amount_cents), 0) as amount_cents").
		Where("status = ? AND completed_at >= ?", "COMPLETED", since).
		Group("DATE(completed_at)").
		Order("date ASC").
		Scan(&points).Error
	return points, err
}

// InteractionsByDay returns daily interaction request counts for the last N days.
func (r *AdminRepository) InteractionsByDay(days int) ([]TimeSeriesPoint, error) {
	since := time.Now().AddDate(0, 0, -days)
	var points []TimeSeriesPoint
	err := r.db.Model(&models.InteractionRequest{}).
		Select("DATE(created_at) as date, COUNT(*) as count").
		Where("created_at >= ?", since).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&points).Error
	return points, err
}

// UpdateUser updates specific fields on a user.
func (r *AdminRepository) UpdateUser(id uint, updates map[string]interface{}) error {
	return r.db.Model(&models.User{}).Where("id = ?", id).Updates(updates).Error
}
