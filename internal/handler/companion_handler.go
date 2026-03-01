package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"lusty/internal/domain"
	"lusty/internal/middleware"
	"lusty/internal/models"
	"lusty/internal/repository"
	"lusty/pkg/cloudinary"

	"github.com/gin-gonic/gin"
)

type CompanionHandler struct {
	repo          *repository.CompanionRepository
	userRepo      *repository.UserRepository
	interactionRepo *repository.InteractionRepository
	cloud         cloudinary.Client
}

func NewCompanionHandler(repo *repository.CompanionRepository, userRepo *repository.UserRepository, interactionRepo *repository.InteractionRepository, cloud cloudinary.Client) *CompanionHandler {
	return &CompanionHandler{repo: repo, userRepo: userRepo, interactionRepo: interactionRepo, cloud: cloud}
}

// GetProfile returns a companion profile by ID (public or own). For CLIENT callers, includes engagement (interaction status) if any, and is_available (false when companion has active sessions).
func (h *CompanionHandler) GetProfile(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	profile, err := h.repo.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
		return
	}
	out := any(profile)
	if h.interactionRepo != nil {
		userID := middleware.GetUserID(c)
		role, _ := c.Get("role")
		if roleStr, _ := role.(string); roleStr == "CLIENT" && userID != 0 {
			profMap, _ := toMap(profile)
			if profMap != nil {
				// Availability: manual toggle (companion sets; auto-off when she accepts a request)
				profMap["is_available"] = profile.IsAvailable
				profMap["availability_status"] = map[bool]string{true: "AVAILABLE", false: "NOT_AVAILABLE"}[profile.IsAvailable]

				// Interested button: inactive only when client has active chat with another companion
				clientBusy, _ := h.interactionRepo.ClientHasActiveSessionWithOtherCompanion(userID, uint(id))
				profMap["client_can_request"] = !clientBusy

				ir, err := h.interactionRepo.GetLatestByClientAndCompanion(userID, uint(id))
				if err == nil && ir != nil {
					engaged := map[string]any{
						"interaction_id":      ir.ID,
						"status":             ir.Status,
						"service_completed":  ir.ServiceCompletedAt != nil,
					}
					if sess, _ := h.interactionRepo.GetChatSessionByInteractionID(ir.ID); sess != nil {
						engaged["session_ends_at"] = sess.EndsAt
						engaged["session_ended"] = sess.EndedAt != nil
					}
					profMap["engagement"] = engaged
				}

				// Add platform markup to pricing for clients
				if pricingRaw, ok := profMap["pricing"]; ok {
					if pricingSlice, ok := pricingRaw.([]interface{}); ok {
						for _, p := range pricingSlice {
							if pm, ok := p.(map[string]interface{}); ok {
								if amt, ok := pm["amount_cents"].(float64); ok {
									pm["amount_cents"] = domain.ClientPrice(int64(amt))
								}
							}
						}
					}
				}

				out = profMap
			}
		}
	}
	c.JSON(http.StatusOK, out)
}

func toMap(v any) (map[string]any, bool) {
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	// Use JSON round-trip for struct->map
	data, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, false
	}
	return m, true
}

// UpdateProfile updates the authenticated companion's profile. Creates one if missing (e.g. legacy COMPANION users).
func (h *CompanionHandler) UpdateProfile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	profile, err := h.repo.GetByUserID(userID)
	if err != nil || profile == nil {
		// Create profile for existing COMPANION users who don't have one yet
		u, uErr := h.userRepo.GetByID(userID)
		if uErr != nil || u == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		displayName := "Companion"
		if u.Email != "" {
			if p, _, ok := strings.Cut(u.Email, "@"); ok && p != "" {
				displayName = p
			}
		}
		profile = &models.CompanionProfile{
			UserID:            userID,
			DisplayName:       displayName,
			AppearInSearch:    false,
			AcceptNewRequests: true,
		}
		if err := h.repo.Create(profile); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "create profile failed"})
			return
		}
	}
	var req struct {
		DisplayName         *string `json:"display_name"`
		Bio                 *string `json:"bio"`
		Interests           *string `json:"interests"`
		Categories         *string `json:"categories"` // comma-separated: tall,slim,long_hair
		Languages           *string `json:"languages"`
		CityOrArea          *string `json:"city_or_area"`
		AvailabilityStatus  *string `json:"availability_status"`
		MainProfileImageURL *string `json:"main_profile_image_url"`
		IsActive            *bool   `json:"is_active"`
		AppearInSearch      *bool   `json:"appear_in_search"`
		AcceptNewRequests   *bool   `json:"accept_new_requests"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.DisplayName != nil {
		profile.DisplayName = *req.DisplayName
	}
	if req.Bio != nil {
		profile.Bio = *req.Bio
	}
	if req.Interests != nil {
		profile.Interests = *req.Interests
	}
	if req.Categories != nil {
		profile.Categories = *req.Categories
	}
	if req.Languages != nil {
		profile.Languages = *req.Languages
	}
	if req.CityOrArea != nil {
		profile.CityOrArea = *req.CityOrArea
	}
	if req.AvailabilityStatus != nil {
		profile.AvailabilityStatus = *req.AvailabilityStatus
	}
	if req.MainProfileImageURL != nil {
		profile.MainProfileImageURL = *req.MainProfileImageURL
	}
	if req.IsActive != nil {
		profile.IsActive = *req.IsActive
	}
	if req.AppearInSearch != nil {
		profile.AppearInSearch = *req.AppearInSearch
	}
	if req.AcceptNewRequests != nil {
		profile.AcceptNewRequests = *req.AcceptNewRequests
	}
	if err := h.repo.Update(profile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	c.JSON(http.StatusOK, profile)
}

// UploadMedia handles image/video upload to Cloudinary (optimized).
func (h *CompanionHandler) UploadMedia(c *gin.Context) {
	userID := middleware.GetUserID(c)
	profile, err := h.repo.GetByUserID(userID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "companion profile required"})
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file required"})
		return
	}
	mediaType := c.DefaultPostForm("media_type", "IMAGE")
	visibility := c.DefaultPostForm("visibility", "PUBLIC")
	folder := "Metchi/companions/" + strconv.FormatUint(uint64(profile.ID), 10)
	prefix := "img"
	if mediaType == "VIDEO" {
		prefix = "vid"
	}
	publicID := prefix + "_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:16]

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not read file"})
		return
	}
	defer f.Close()

	ctx := c.Request.Context()
	var url, thumbURL string
	if mediaType == "VIDEO" {
		url, thumbURL, err = h.cloud.UploadVideo(ctx, f, folder, publicID)
	} else {
		url, thumbURL, err = h.cloud.UploadImage(ctx, f, folder, publicID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upload failed"})
		return
	}
	media := &models.CompanionMedia{
		CompanionID:  profile.ID,
		MediaType:    mediaType,
		URL:          url,
		ThumbnailURL: thumbURL,
		Visibility:   visibility,
	}
	if err := h.repo.AddMedia(media); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save failed"})
		return
	}
	c.JSON(http.StatusCreated, media)
}
