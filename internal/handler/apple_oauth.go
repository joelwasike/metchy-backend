package handler

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"lusty/internal/domain"
	"lusty/internal/models"
	"lusty/internal/repository"
	"lusty/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type AppleOAuthHandler struct {
	authSvc       *service.AuthService
	presenceRepo  *repository.PresenceRepository
	auditRepo     *repository.AuditLogRepository
	companionRepo *repository.CompanionRepository
	referralSvc   *service.ReferralService
}

func NewAppleOAuthHandler(
	authSvc *service.AuthService,
	presenceRepo *repository.PresenceRepository,
	auditRepo *repository.AuditLogRepository,
	companionRepo *repository.CompanionRepository,
	referralSvc *service.ReferralService,
) *AppleOAuthHandler {
	return &AppleOAuthHandler{
		authSvc:       authSvc,
		presenceRepo:  presenceRepo,
		auditRepo:     auditRepo,
		companionRepo: companionRepo,
		referralSvc:   referralSvc,
	}
}

// appleKeysCache caches Apple's public keys to avoid fetching on every request.
var (
	appleKeysMu    sync.Mutex
	appleKeysCache map[string]*rsa.PublicKey
	appleKeysTTL   time.Time
)

// fetchApplePublicKeys fetches Apple's JWKS and returns RSA public keys keyed by kid.
func fetchApplePublicKeys() (map[string]*rsa.PublicKey, error) {
	appleKeysMu.Lock()
	defer appleKeysMu.Unlock()
	if appleKeysCache != nil && time.Now().Before(appleKeysTTL) {
		return appleKeysCache, nil
	}
	resp, err := http.Get("https://appleid.apple.com/auth/keys")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var jwks struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, err
	}
	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" {
			continue
		}
		nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			continue
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			continue
		}
		n := new(big.Int).SetBytes(nBytes)
		e := 0
		for _, b := range eBytes {
			e = e<<8 + int(b)
		}
		keys[k.Kid] = &rsa.PublicKey{N: n, E: e}
	}
	appleKeysCache = keys
	appleKeysTTL = time.Now().Add(1 * time.Hour)
	return keys, nil
}

// Token accepts an identity_token from the Flutter sign_in_with_apple package,
// verifies it against Apple's public keys, and returns JWT tokens.
// Optional fields: role (CLIENT|COMPANION), referral_code, name — only applied for new users.
func (h *AppleOAuthHandler) Token(c *gin.Context) {
	var req struct {
		IdentityToken string `json:"identity_token" binding:"required"`
		Role          string `json:"role"`
		ReferralCode  string `json:"referral_code"`
		Name          string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "identity_token required"})
		return
	}

	// Parse the JWT header to get the kid
	parts := strings.SplitN(req.IdentityToken, ".", 3)
	if len(parts) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed identity_token"})
		return
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token header"})
		return
	}
	var header struct {
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil || header.Kid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing kid in token"})
		return
	}

	keys, err := fetchApplePublicKeys()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch Apple keys"})
		return
	}
	pubKey, ok := keys[header.Kid]
	if !ok {
		// Invalidate cache and retry once
		appleKeysMu.Lock()
		appleKeysTTL = time.Time{}
		appleKeysMu.Unlock()
		keys, err = fetchApplePublicKeys()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch Apple keys"})
			return
		}
		pubKey, ok = keys[header.Kid]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown signing key"})
			return
		}
	}

	// Verify the token
	token, err := jwt.Parse(req.IdentityToken, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return pubKey, nil
	})
	if err != nil || !token.Valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid identity_token"})
		return
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token claims"})
		return
	}
	// Verify issuer
	iss, _ := claims["iss"].(string)
	if iss != "https://appleid.apple.com" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid issuer"})
		return
	}
	sub, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)
	if sub == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing sub in token"})
		return
	}

	u, access, refresh, isNew, roleChanged, err := h.authSvc.LoginWithApple(sub, email, req.Name, req.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		return
	}

	// Create companion profile for new users or when role switched to COMPANION
	if (isNew || roleChanged) && u.Role == domain.RoleCompanion && h.companionRepo != nil {
		displayName := u.Username
		if displayName == "" {
			displayName, _, _ = strings.Cut(u.Email, "@")
		}
		if displayName == "" {
			displayName = "Companion"
		}
		_ = h.companionRepo.Create(&models.CompanionProfile{
			UserID:            u.ID,
			DisplayName:       displayName,
			AppearInSearch:    false,
			AcceptNewRequests: true,
		})
	}
	// Process referral code only for brand-new accounts (creates referral + bonus for companions)
	if isNew && h.referralSvc != nil {
		h.referralSvc.ProcessReferralCode(req.ReferralCode, u)
	}

	presence, _ := h.presenceRepo.GetByUserID(u.ID)
	if presence == nil {
		presence = &models.UserPresence{UserID: u.ID}
	}
	presence.Status = domain.PresenceOnline
	presence.IsOnline = true
	presence.LastSeenAt = time.Now()
	_ = h.presenceRepo.Upsert(presence)
	_ = h.auditRepo.Create(&models.AuditLog{
		UserID:    &u.ID,
		Action:    fmt.Sprintf("apple_oauth_token isNew=%v", isNew),
		Resource:  "auth",
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	})
	c.JSON(http.StatusOK, gin.H{"user": u, "access_token": access, "refresh_token": refresh})
}
