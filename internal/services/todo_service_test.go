package services

import (
	"errors"
	"testing"

	"go-backend/internal/models"
)

func TestTodoCRUDAsOwner(t *testing.T) {
	db := newTestDB(t)
	svc := NewTodoService(db)
	owner := models.User{Email: "owner@x.com", PasswordHash: "h", Role: models.RoleUser}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatalf("create owner: %v", err)
	}
	todo, err := svc.Create(owner.ID, "Buy milk", "from the shop")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if todo.Title != "Buy milk" || todo.OwnerID != owner.ID {
		t.Errorf("unexpected todo: %+v", todo)
	}
	got, err := svc.Get(todo.ID, owner.ID, false)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != todo.ID {
		t.Errorf("expected todo %d, got %d", todo.ID, got.ID)
	}
	updated, err := svc.Update(todo.ID, owner.ID, false, map[string]interface{}{"done": true})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !updated.Done {
		t.Error("expected done to be true after update")
	}
	if err := svc.Delete(todo.ID, owner.ID, false); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestTodoAccessDeniedForNonOwner(t *testing.T) {
	db := newTestDB(t)
	svc := NewTodoService(db)
	owner := models.User{Email: "owner@x.com", PasswordHash: "h", Role: models.RoleUser}
	other := models.User{Email: "other@x.com", PasswordHash: "h", Role: models.RoleUser}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	todo := models.Todo{Title: "secret", OwnerID: owner.ID}
	if err := db.Create(&todo).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(todo.ID, other.ID, false); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for non-owner, got %v", err)
	}
	if _, err := svc.Get(todo.ID, other.ID, true); err != nil {
		t.Errorf("expected admin to access, got %v", err)
	}
	if err := svc.Delete(todo.ID, other.ID, false); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound on non-owner delete, got %v", err)
	}
}

func TestTodoUpdateDeniedForNonOwner(t *testing.T) {
	db := newTestDB(t)
	svc := NewTodoService(db)
	owner := models.User{Email: "owner@x.com", PasswordHash: "h", Role: models.RoleUser}
	other := models.User{Email: "other@x.com", PasswordHash: "h", Role: models.RoleUser}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	todo := models.Todo{Title: "secret", OwnerID: owner.ID}
	if err := db.Create(&todo).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Update(todo.ID, other.ID, false, map[string]interface{}{"title": "hacked"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound on non-owner update, got %v", err)
	}
	if err := svc.Delete(todo.ID, other.ID, false); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound on non-owner delete, got %v", err)
	}
}

func TestTodoListByOwner(t *testing.T) {
	db := newTestDB(t)
	svc := NewTodoService(db)
	a := models.User{Email: "a@x.com", PasswordHash: "h", Role: models.RoleUser}
	b := models.User{Email: "b@x.com", PasswordHash: "h", Role: models.RoleUser}
	if err := db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&b).Error; err != nil {
		t.Fatal(err)
	}
	for _, ownerID := range []uint{a.ID, a.ID, b.ID} {
		if err := db.Create(&models.Todo{Title: "t", OwnerID: ownerID}).Error; err != nil {
			t.Fatal(err)
		}
	}
	list, err := svc.ListByOwner(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 todos for a, got %d", len(list))
	}
}
