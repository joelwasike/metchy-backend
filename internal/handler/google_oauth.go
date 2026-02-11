package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"lusty/config"
	"lusty/internal/domain"
	"lusty/internal/models"
	"lusty/internal/repository"
	"lusty/internal/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleOAuthHandler struct {
	cfg       *config.Config
	authSvc   *service.AuthService
	presenceRepo *repository.PresenceRepository
	auditRepo *repository.AuditLogRepository
}

func NewGoogleOAuthHandler(cfg *config.Config, authSvc *service.AuthService, presenceRepo *repository.PresenceRepository, auditRepo *repository.AuditLogRepository) *GoogleOAuthHandler {
	return &GoogleOAuthHandler{cfg: cfg, authSvc: authSvc, presenceRepo: presenceRepo, auditRepo: auditRepo}
}

func (h *GoogleOAuthHandler) OAuth2Config() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     h.cfg.OAuth.GoogleClientID,
		ClientSecret: h.cfg.OAuth.GoogleClientSecret,
		RedirectURL:  h.cfg.OAuth.GoogleRedirectURL,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
		Endpoint:     google.Endpoint,
	}
}

// Redirect redirects user to Google consent screen.
func (h *GoogleOAuthHandler) Redirect(c *gin.Context) {
	if h.cfg.OAuth.GoogleClientID == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Google OAuth not configured"})
		return
	}
	url := h.OAuth2Config().AuthCodeURL("state", oauth2.AccessTypeOffline)
	c.Redirect(http.StatusFound, url)
}

type googleUserInfo struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// Callback exchanges code for tokens, fetches user info, creates/links user, returns JWT.
func (h *GoogleOAuthHandler) Callback(c *gin.Context) {
	if h.cfg.OAuth.GoogleClientID == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Google OAuth not configured"})
		return
	}
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing code"})
		return
	}
	ctx := c.Request.Context()
	conf := h.OAuth2Config()
	tok, err := conf.Exchange(ctx, code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "exchange failed"})
		return
	}
	client := conf.Client(ctx, tok)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil || resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user info"})
		return
	}
	defer resp.Body.Close()
	var info googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user info"})
		return
	}
	u, access, refresh, err := h.authSvc.LoginWithGoogle(info.ID, info.Email, info.Name, info.Picture)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		return
	}
	// Set presence ONLINE
	presence, _ := h.presenceRepo.GetByUserID(u.ID)
	if presence == nil {
		presence = &models.UserPresence{UserID: u.ID}
	}
	presence.Status = domain.PresenceOnline
	presence.IsOnline = true
	_ = h.presenceRepo.Upsert(presence)
	// Audit
	_ = h.auditRepo.Create(&models.AuditLog{UserID: &u.ID, Action: "google_oauth_login", Resource: "auth", IP: c.ClientIP(), UserAgent: c.Request.UserAgent()})
	c.JSON(http.StatusOK, gin.H{"user": u, "access_token": access, "refresh_token": refresh})
}

// tokeninfoResponse is the response from https://oauth2.googleapis.com/tokeninfo?id_token=...
type tokeninfoResponse struct {
	Sub     string `json:"sub"`     // Google ID
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// Token accepts an ID token from mobile (google_sign_in) and returns JWT. For Flutter/iOS/Android.
func (h *GoogleOAuthHandler) Token(c *gin.Context) {
	if h.cfg.OAuth.GoogleClientID == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Google OAuth not configured"})
		return
	}
	var req struct {
		IDToken string `json:"id_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id_token required"})
		return
	}
	resp, err := http.Get("https://oauth2.googleapis.com/tokeninfo?id_token=" + url.QueryEscape(req.IDToken))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token verification failed"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id_token", "detail": string(body)})
		return
	}
	var info tokeninfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid token response"})
		return
	}
	if info.Sub == "" || info.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token payload"})
		return
	}
	u, access, refresh, err := h.authSvc.LoginWithGoogle(info.Sub, info.Email, info.Name, info.Picture)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		return
	}
	presence, _ := h.presenceRepo.GetByUserID(u.ID)
	if presence == nil {
		presence = &models.UserPresence{UserID: u.ID}
	}
	presence.Status = domain.PresenceOnline
	presence.IsOnline = true
	_ = h.presenceRepo.Upsert(presence)
	_ = h.auditRepo.Create(&models.AuditLog{UserID: &u.ID, Action: "google_oauth_token", Resource: "auth", IP: c.ClientIP(), UserAgent: c.Request.UserAgent()})
	c.JSON(http.StatusOK, gin.H{"user": u, "access_token": access, "refresh_token": refresh})
}
