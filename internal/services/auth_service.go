package services

import (
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"go-backend/internal/models"
	"go-backend/pkg/token"
)

type AuthService struct {
	db         *gorm.DB
	secret     string
	tokenHours int
}

func NewAuthService(db *gorm.DB, secret string, tokenHours int) *AuthService {
	return &AuthService{db: db, secret: secret, tokenHours: tokenHours}
}

func (s *AuthService) Register(email, password string) (*models.User, error) {
	email = normalizeEmail(email)
	var count int64
	if err := s.db.Model(&models.User{}).Where("email = ?", email).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrEmailTaken
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := &models.User{Email: email, PasswordHash: string(hash), Role: models.RoleUser}
	if err := s.db.Create(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func (s *AuthService) Login(email, password string) (string, error) {
	var user models.User
	if err := s.db.Where("email = ?", normalizeEmail(email)).First(&user).Error; err != nil {
		return "", ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", ErrInvalidCredentials
	}
	return s.GenerateToken(&user)
}

func (s *AuthService) GenerateToken(user *models.User) (string, error) {
	return token.Generate(user.ID, user.Role, s.secret, s.tokenHours)
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
