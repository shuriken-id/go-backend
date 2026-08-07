package services

import (
	"testing"

	"go-backend/internal/models"
)

func TestUserService(t *testing.T) {
	db := newTestDB(t)
	svc := NewUserService(db)
	alice := models.User{Email: "alice@x.com", PasswordHash: "h", Role: models.RoleUser}
	bob := models.User{Email: "bob@x.com", PasswordHash: "h", Role: models.RoleAdmin}
	if err := db.Create(&alice).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&bob).Error; err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetByID(alice.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.Email != alice.Email {
		t.Errorf("expected %s, got %s", alice.Email, got.Email)
	}
	users, err := svc.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
}
