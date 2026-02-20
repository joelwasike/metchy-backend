package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"lusty/config"
	"lusty/internal/auth"
	"lusty/internal/domain"
	"lusty/internal/models"
	"lusty/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrEmailExists    = errors.New("email already registered")
	ErrUsernameExists = errors.New("username already taken")
	ErrInvalidCreds   = errors.New("invalid email or password")
	ErrAgeRequired    = errors.New("must be 18 or older")
)

type AuthService struct {
	cfg      *config.Config
	userRepo *repository.UserRepository
}

func NewAuthService(cfg *config.Config, userRepo *repository.UserRepository) *AuthService {
	return &AuthService{cfg: cfg, userRepo: userRepo}
}

func (s *AuthService) Register(email, username, password, role string, dateOfBirth time.Time) (*models.User, string, string, error) {
	if s.cfg.Location.MinAge < 18 {
		s.cfg.Location.MinAge = 18
	}
	age := time.Now().Year() - dateOfBirth.Year()
	if time.Now().YearDay() < dateOfBirth.YearDay() {
		age--
	}
	if age < s.cfg.Location.MinAge {
		return nil, "", "", ErrAgeRequired
	}
	_, err := s.userRepo.GetByEmail(email)
	if err == nil {
		return nil, "", "", ErrEmailExists
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", "", err
	}
	_, err = s.userRepo.GetByUsername(username)
	if err == nil {
		return nil, "", "", ErrUsernameExists
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", "", err
	}
	u := &models.User{
		Email:        email,
		Username:     username,
		PasswordHash: string(hash),
		Role:         role,
		DateOfBirth:  &dateOfBirth,
	}
	if err := s.userRepo.Create(u); err != nil {
		return nil, "", "", err
	}
	access, err := auth.GenerateAccessToken(&s.cfg.JWT, u.ID, u.Email, u.Role)
	if err != nil {
		return u, "", "", err
	}
	refresh, err := auth.GenerateRefreshToken(&s.cfg.JWT, u.ID)
	if err != nil {
		return u, access, "", err
	}
	return u, access, refresh, nil
}

func (s *AuthService) Login(email, password string) (*models.User, string, string, error) {
	u, err := s.userRepo.GetByEmail(email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", "", ErrInvalidCreds
		}
		return nil, "", "", err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, "", "", ErrInvalidCreds
	}
	access, _ := auth.GenerateAccessToken(&s.cfg.JWT, u.ID, u.Email, u.Role)
	refresh, _ := auth.GenerateRefreshToken(&s.cfg.JWT, u.ID)
	return u, access, refresh, nil
}

// LoginWithGoogle creates or finds user by Google ID and returns user + tokens + isNew flag.
// role is only applied when creating a brand new user; pass empty string to default to CLIENT.
func (s *AuthService) LoginWithGoogle(googleID, email, name, avatarURL, role string) (*models.User, string, string, bool, error) {
	u, err := s.userRepo.GetByGoogleID(googleID)
	if err == nil {
		access, _ := auth.GenerateAccessToken(&s.cfg.JWT, u.ID, u.Email, u.Role)
		refresh, _ := auth.GenerateRefreshToken(&s.cfg.JWT, u.ID)
		return u, access, refresh, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", "", false, err
	}
	// New user: check email not already used
	existing, _ := s.userRepo.GetByEmail(email)
	if existing != nil {
		// Link Google to existing account
		gid := googleID
		existing.GoogleID = &gid
		if avatarURL != "" {
			existing.AvatarURL = avatarURL
		}
		if err := s.userRepo.Update(existing); err != nil {
			return nil, "", "", false, err
		}
		access, _ := auth.GenerateAccessToken(&s.cfg.JWT, existing.ID, existing.Email, existing.Role)
		refresh, _ := auth.GenerateRefreshToken(&s.cfg.JWT, existing.ID)
		return existing, access, refresh, false, nil
	}
	// Create new user; validate role
	if role != domain.RoleCompanion {
		role = domain.RoleClient
	}
	gid := googleID
	username := strings.Split(email, "@")[0]
	if name != "" {
		username = strings.ReplaceAll(strings.ToLower(name), " ", "_")
	}
	if username == "" {
		username = "user" + fmt.Sprintf("%d", time.Now().UnixNano()%100000)
	}
	u = &models.User{
		Email:       email,
		Username:    username,
		GoogleID:    &gid,
		Role:        role,
		AvatarURL:   avatarURL,
		DateOfBirth: nil,
	}
	if err := s.userRepo.Create(u); err != nil {
		return nil, "", "", false, err
	}
	access, _ := auth.GenerateAccessToken(&s.cfg.JWT, u.ID, u.Email, u.Role)
	refresh, _ := auth.GenerateRefreshToken(&s.cfg.JWT, u.ID)
	return u, access, refresh, true, nil
}

// ChangePassword updates the user's password. Requires current password verification.
func (s *AuthService) ChangePassword(userID uint, currentPassword, newPassword string) error {
	u, err := s.userRepo.GetByID(userID)
	if err != nil || u == nil {
		return ErrInvalidCreds
	}
	if u.PasswordHash == "" {
		return errors.New("account uses Google sign-in; set a password first")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(currentPassword)); err != nil {
		return ErrInvalidCreds
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return s.userRepo.Update(u)
}

func (s *AuthService) RefreshToken(refreshToken string) (access, refresh string, err error) {
	token, err := jwt.ParseWithClaims(refreshToken, &jwt.RegisteredClaims{}, func(_ *jwt.Token) (interface{}, error) {
		return []byte(s.cfg.JWT.RefreshSecret), nil
	})
	if err != nil || !token.Valid {
		return "", "", auth.ErrInvalidToken
	}
	claims := token.Claims.(*jwt.RegisteredClaims)
	var userID uint
	fmt.Sscanf(claims.Subject, "%d", &userID)
	u, err := s.userRepo.GetByID(userID)
	if err != nil {
		return "", "", err
	}
	access, _ = auth.GenerateAccessToken(&s.cfg.JWT, u.ID, u.Email, u.Role)
	refresh, _ = auth.GenerateRefreshToken(&s.cfg.JWT, u.ID)
	return access, refresh, nil
}
