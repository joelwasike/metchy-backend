package handler

import (
	"net/http"
	"strconv"

	"lusty/internal/repository"
	"lusty/internal/service"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	adminRepo   *repository.AdminRepository
	settingRepo *repository.SettingRepository
	authSvc     *service.AuthService
}

func NewAdminHandler(
	adminRepo *repository.AdminRepository,
	settingRepo *repository.SettingRepository,
	authSvc *service.AuthService,
) *AdminHandler {
	return &AdminHandler{
		adminRepo:   adminRepo,
		settingRepo: settingRepo,
		authSvc:     authSvc,
	}
}

// AdminLogin handles POST /admin/login — admin-only login.
func (h *AdminHandler) AdminLogin(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	u, access, refresh, err := h.authSvc.Login(req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if u.Role != "ADMIN" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user":          u,
		"access_token":  access,
		"refresh_token": refresh,
	})
}

// Dashboard handles GET /admin/dashboard — overview stats.
func (h *AdminHandler) Dashboard(c *gin.Context) {
	stats, err := h.adminRepo.GetDashboardStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load stats"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// ListUsers handles GET /admin/users.
func (h *AdminHandler) ListUsers(c *gin.Context) {
	search := c.Query("search")
	role := c.Query("role")
	page, limit := parsePagination(c)
	users, total, err := h.adminRepo.ListUsers(search, role, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list users"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": users, "total": total, "page": page, "limit": limit})
}

// GetUser handles GET /admin/users/:id.
func (h *AdminHandler) GetUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	u, err := h.adminRepo.GetUserByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, u)
}

// UpdateUser handles PATCH /admin/users/:id.
func (h *AdminHandler) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Only allow safe fields
	allowed := map[string]bool{"role": true, "kyc": true, "username": true, "email": true}
	safe := make(map[string]interface{})
	for k, v := range updates {
		if allowed[k] {
			safe[k] = v
		}
	}
	if len(safe) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no valid fields to update"})
		return
	}
	if err := h.adminRepo.UpdateUser(uint(id), safe); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ListCompanions handles GET /admin/companions.
func (h *AdminHandler) ListCompanions(c *gin.Context) {
	search := c.Query("search")
	page, limit := parsePagination(c)
	list, total, err := h.adminRepo.ListCompanions(search, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list companions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list, "total": total, "page": page, "limit": limit})
}

// ListTransactions handles GET /admin/transactions.
func (h *AdminHandler) ListTransactions(c *gin.Context) {
	txType := c.Query("type")
	page, limit := parsePagination(c)
	list, total, err := h.adminRepo.ListTransactions(txType, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list transactions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list, "total": total, "page": page, "limit": limit})
}

// ListPayments handles GET /admin/payments.
func (h *AdminHandler) ListPayments(c *gin.Context) {
	status := c.Query("status")
	page, limit := parsePagination(c)
	list, total, err := h.adminRepo.ListPayments(status, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list payments"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list, "total": total, "page": page, "limit": limit})
}

// ListWithdrawals handles GET /admin/withdrawals.
func (h *AdminHandler) ListWithdrawals(c *gin.Context) {
	status := c.Query("status")
	page, limit := parsePagination(c)
	list, total, err := h.adminRepo.ListWithdrawals(status, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list withdrawals"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list, "total": total, "page": page, "limit": limit})
}

// ListInteractions handles GET /admin/interactions.
func (h *AdminHandler) ListInteractions(c *gin.Context) {
	status := c.Query("status")
	page, limit := parsePagination(c)
	list, total, err := h.adminRepo.ListInteractions(status, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list interactions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list, "total": total, "page": page, "limit": limit})
}

// ListReports handles GET /admin/reports.
func (h *AdminHandler) ListReports(c *gin.Context) {
	status := c.Query("status")
	page, limit := parsePagination(c)
	list, total, err := h.adminRepo.ListReports(status, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list reports"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list, "total": total, "page": page, "limit": limit})
}

// UpdateReport handles PATCH /admin/reports/:id.
func (h *AdminHandler) UpdateReport(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Status string `json:"status" binding:"required,oneof=REVIEWED RESOLVED"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.adminRepo.UpdateReportStatus(uint(id), req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ListReferrals handles GET /admin/referrals.
func (h *AdminHandler) ListReferrals(c *gin.Context) {
	page, limit := parsePagination(c)
	list, total, err := h.adminRepo.ListReferrals(page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list referrals"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list, "total": total, "page": page, "limit": limit})
}

// ListOnlineUsers handles GET /admin/online-users.
func (h *AdminHandler) ListOnlineUsers(c *gin.Context) {
	page, limit := parsePagination(c)
	list, total, err := h.adminRepo.ListOnlineUsers(page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list online users"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list, "total": total, "page": page, "limit": limit})
}

// GetSettings handles GET /admin/settings.
func (h *AdminHandler) GetSettings(c *gin.Context) {
	settings, err := h.settingRepo.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load settings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": settings})
}

// UpdateSettings handles PUT /admin/settings.
func (h *AdminHandler) UpdateSettings(c *gin.Context) {
	var req struct {
		Settings map[string]string `json:"settings" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	for k, v := range req.Settings {
		if err := h.settingRepo.Set(k, v); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update setting: " + k})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Analytics handles GET /admin/analytics?days=30.
func (h *AdminHandler) Analytics(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days <= 0 || days > 365 {
		days = 30
	}
	signups, _ := h.adminRepo.UserSignupsByDay(days)
	revenue, _ := h.adminRepo.RevenueByDay(days)
	interactions, _ := h.adminRepo.InteractionsByDay(days)
	c.JSON(http.StatusOK, gin.H{
		"signups":      signups,
		"revenue":      revenue,
		"interactions": interactions,
		"days":         days,
	})
}

func parsePagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return page, limit
}
