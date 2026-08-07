package services

import (
	"errors"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"go-backend/internal/models"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Todo{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return db
}

func TestRegisterNormalizesEmail(t *testing.T) {
	svc := NewAuthService(newTestDB(t), "secret", 24)
	u, err := svc.Register("Alice@Example.com", "password123")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if u.Email != "alice@example.com" {
		t.Errorf("expected normalized email, got %q", u.Email)
	}
	if u.Role != models.RoleUser {
		t.Errorf("expected role user, got %q", u.Role)
	}
	if u.PasswordHash == "" || u.PasswordHash == "password123" {
		t.Error("password must be hashed")
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	svc := NewAuthService(newTestDB(t), "secret", 24)
	if _, err := svc.Register("a@b.com", "password123"); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if _, err := svc.Register("a@b.com", "password123"); !errors.Is(err, ErrEmailTaken) {
		t.Errorf("expected ErrEmailTaken, got %v", err)
	}
}

func TestLoginSuccess(t *testing.T) {
	svc := NewAuthService(newTestDB(t), "secret", 24)
	if _, err := svc.Register("a@b.com", "password123"); err != nil {
		t.Fatalf("register: %v", err)
	}
	tk, err := svc.Login("a@b.com", "password123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if strings.TrimSpace(tk) == "" {
		t.Error("expected a non-empty token")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	svc := NewAuthService(newTestDB(t), "secret", 24)
	if _, err := svc.Register("a@b.com", "password123"); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := svc.Login("a@b.com", "wrongpass"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}
