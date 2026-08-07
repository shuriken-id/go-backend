package services

import (
	"gorm.io/gorm"

	"go-backend/internal/models"
)

type UserService struct {
	db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

func (s *UserService) GetByID(id uint) (*models.User, error) {
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *UserService) List() ([]models.User, error) {
	var users []models.User
	if err := s.db.Order("created_at asc").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}