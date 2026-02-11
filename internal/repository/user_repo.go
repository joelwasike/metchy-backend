package repository

import (
	"lusty/internal/models"

	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(u *models.User) error {
	return r.db.Create(u).Error
}

func (r *UserRepository) GetByID(id uint) (*models.User, error) {
	var u models.User
	err := r.db.First(&u, id).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	var u models.User
	err := r.db.Where("email = ?", email).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) GetByUsername(username string) (*models.User, error) {
	var u models.User
	err := r.db.Where("username = ?", username).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) GetByGoogleID(googleID string) (*models.User, error) {
	var u models.User
	err := r.db.Where("google_id = ?", googleID).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) Update(u *models.User) error {
	return r.db.Save(u).Error
}
