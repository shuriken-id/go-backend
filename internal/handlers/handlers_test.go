package handlers_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"go-backend/internal/models"
	"go-backend/internal/router"
	"go-backend/pkg/config"
	"go-backend/pkg/token"
)

func formatID(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}

func newTestRouter(t *testing.T) (*fiber.App, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Todo{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cfg := &config.Config{JWTSecret: "test-secret", TokenHours: 24}
	return router.New(db, cfg), db
}

func tokenFor(t *testing.T, u *models.User) string {
	t.Helper()
	tk, err := token.Generate(u.ID, u.Role, "test-secret", 24)
	if err != nil {
		t.Fatal(err)
	}
	return tk
}

func seedUser(t *testing.T, db *gorm.DB, email, role string) (*models.User, string) {
	t.Helper()
	u := &models.User{Email: email, PasswordHash: "h", Role: role}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	return u, tokenFor(t, u)
}

func request(t *testing.T, app *fiber.App, method, path, token string, body interface{}) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestHealthz(t *testing.T) {
	r, _ := newTestRouter(t)
	resp := request(t, r, http.MethodGet, "/healthz", "", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuthFlow(t *testing.T) {
	r, _ := newTestRouter(t)
	if resp := request(t, r, http.MethodPost, "/api/v1/auth/register", "", map[string]string{
		"email": "new@example.com", "password": "password123",
	}); resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("register expected 201, got %d: %s", resp.StatusCode, string(body))
	} else {
		resp.Body.Close()
	}
	if resp := request(t, r, http.MethodPost, "/api/v1/auth/register", "", map[string]string{
		"email": "NEW@example.com", "password": "password123",
	}); resp.StatusCode != http.StatusConflict {
		t.Errorf("duplicate register expected 409, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
	if resp := request(t, r, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email": "new@example.com", "password": "password123",
	}); resp.StatusCode != http.StatusOK {
		t.Errorf("login expected 200, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
	if resp := request(t, r, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email": "new@example.com", "password": "wrong",
	}); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("bad login expected 401, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
}

func TestTodoOwnership(t *testing.T) {
	r, db := newTestRouter(t)
	_, ownerTk := seedUser(t, db, "owner@x.com", models.RoleUser)
	other := models.User{Email: "other@x.com", PasswordHash: "h", Role: models.RoleUser}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	otherTk := tokenFor(t, &other)

	if resp := request(t, r, http.MethodGet, "/api/v1/todos", "", nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no token expected 401, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}

	var created models.Todo
	if resp := request(t, r, http.MethodPost, "/api/v1/todos", ownerTk, map[string]string{
		"title": "first", "description": "desc",
	}); resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("create expected 201, got %d: %s", resp.StatusCode, string(body))
	} else {
		if err := json.Unmarshal(mustRead(t, resp), &created); err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	path := "/api/v1/todos/" + formatID(created.ID)

	if resp := request(t, r, http.MethodGet, path, otherTk, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("non-owner get expected 404, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
	if resp := request(t, r, http.MethodGet, path, ownerTk, nil); resp.StatusCode != http.StatusOK {
		t.Errorf("owner get expected 200, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
	if resp := request(t, r, http.MethodDelete, path, otherTk, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("non-owner delete expected 404, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
	if resp := request(t, r, http.MethodDelete, path, ownerTk, nil); resp.StatusCode != http.StatusNoContent {
		t.Errorf("owner delete expected 204, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
}

func TestAdminRoutes(t *testing.T) {
	r, db := newTestRouter(t)
	_, userTk := seedUser(t, db, "u@x.com", models.RoleUser)
	_, adminTk := seedUser(t, db, "a@x.com", models.RoleAdmin)
	if resp := request(t, r, http.MethodGet, "/api/v1/users/me", userTk, nil); resp.StatusCode != http.StatusOK {
		t.Errorf("me expected 200, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
	if resp := request(t, r, http.MethodGet, "/api/v1/users", userTk, nil); resp.StatusCode != http.StatusForbidden {
		t.Errorf("role guard expected 403, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
	if resp := request(t, r, http.MethodGet, "/api/v1/users", adminTk, nil); resp.StatusCode != http.StatusOK {
		t.Errorf("admin list expected 200, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
}

func mustRead(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
