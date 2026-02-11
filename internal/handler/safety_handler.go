package handler

import (
	"net/http"
	"strconv"

	"lusty/internal/middleware"
	"lusty/internal/models"
	"lusty/internal/repository"

	"github.com/gin-gonic/gin"
)

type BlockHandler struct {
	repo *repository.BlockRepository
}

func NewBlockHandler(repo *repository.BlockRepository) *BlockHandler {
	return &BlockHandler{repo: repo}
}

func (h *BlockHandler) Block(c *gin.Context) {
	blockerID := middleware.GetUserID(c)
	blockedID, _ := strconv.ParseUint(c.Param("user_id"), 10, 64)
	if blockerID == uint(blockedID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot block yourself"})
		return
	}
	if err := h.repo.Create(&models.Block{BlockerID: blockerID, BlockedID: uint(blockedID)}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to block"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *BlockHandler) Unblock(c *gin.Context) {
	blockerID := middleware.GetUserID(c)
	blockedID, _ := strconv.ParseUint(c.Param("user_id"), 10, 64)
	if err := h.repo.Delete(blockerID, uint(blockedID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to unblock"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type ReportHandler struct {
	repo      *repository.ReportRepository
	auditRepo *repository.AuditLogRepository
}

func NewReportHandler(repo *repository.ReportRepository, auditRepo *repository.AuditLogRepository) *ReportHandler {
	return &ReportHandler{repo: repo, auditRepo: auditRepo}
}

func (h *ReportHandler) Create(c *gin.Context) {
	reporterID := middleware.GetUserID(c)
	var req struct {
		ReportedID uint   `json:"reported_id" binding:"required"`
		Reason     string `json:"reason"`
		Details    string `json:"details"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if reporterID == req.ReportedID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot report yourself"})
		return
	}
	report := &models.Report{
		ReporterID: reporterID,
		ReportedID: req.ReportedID,
		Reason:     req.Reason,
		Details:    req.Details,
		Status:     "PENDING",
	}
	if err := h.repo.Create(report); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to submit report"})
		return
	}
	if h.auditRepo != nil {
		_ = h.auditRepo.Create(&models.AuditLog{
			UserID:     &reporterID,
			Action:     "report_create",
			Resource:   "report",
			ResourceID: strconv.FormatUint(uint64(report.ID), 10),
			IP:         c.ClientIP(),
			UserAgent:  c.Request.UserAgent(),
		})
	}
	c.JSON(http.StatusCreated, report)
}
