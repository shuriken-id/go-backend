package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"go-backend/internal/models"
	"go-backend/internal/router"
	"go-backend/pkg/config"
	"go-backend/pkg/token"
)

func formatID(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}

func newTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
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

func request(t *testing.T, h http.Handler, method, path, token string, body interface{}) *httptest.ResponseRecorder {
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
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealthz(t *testing.T) {
	r, _ := newTestRouter(t)
	if rec := request(t, r, http.MethodGet, "/healthz", "", nil); rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAuthFlow(t *testing.T) {
	r, _ := newTestRouter(t)
	if rec := request(t, r, http.MethodPost, "/api/v1/auth/register", "", map[string]string{
		"email": "new@example.com", "password": "password123",
	}); rec.Code != http.StatusCreated {
		t.Errorf("register expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec := request(t, r, http.MethodPost, "/api/v1/auth/register", "", map[string]string{
		"email": "NEW@example.com", "password": "password123",
	}); rec.Code != http.StatusConflict {
		t.Errorf("duplicate register expected 409, got %d", rec.Code)
	}
	if rec := request(t, r, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email": "new@example.com", "password": "password123",
	}); rec.Code != http.StatusOK {
		t.Errorf("login expected 200, got %d", rec.Code)
	}
	if rec := request(t, r, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email": "new@example.com", "password": "wrong",
	}); rec.Code != http.StatusUnauthorized {
		t.Errorf("bad login expected 401, got %d", rec.Code)
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

	if rec := request(t, r, http.MethodGet, "/api/v1/todos", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("no token expected 401, got %d", rec.Code)
	}

	var created models.Todo
	if rec := request(t, r, http.MethodPost, "/api/v1/todos", ownerTk, map[string]string{
		"title": "first", "description": "desc",
	}); rec.Code != http.StatusCreated {
		t.Fatalf("create expected 201, got %d: %s", rec.Code, rec.Body.String())
	} else {
		if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
			t.Fatal(err)
		}
	}
	path := "/api/v1/todos/" + formatID(created.ID)

	if rec := request(t, r, http.MethodGet, path, otherTk, nil); rec.Code != http.StatusNotFound {
		t.Errorf("non-owner get expected 404, got %d", rec.Code)
	}
	if rec := request(t, r, http.MethodGet, path, ownerTk, nil); rec.Code != http.StatusOK {
		t.Errorf("owner get expected 200, got %d", rec.Code)
	}
	if rec := request(t, r, http.MethodDelete, path, otherTk, nil); rec.Code != http.StatusNotFound {
		t.Errorf("non-owner delete expected 404, got %d", rec.Code)
	}
	if rec := request(t, r, http.MethodDelete, path, ownerTk, nil); rec.Code != http.StatusNoContent {
		t.Errorf("owner delete expected 204, got %d", rec.Code)
	}
}

func TestAdminRoutes(t *testing.T) {
	r, db := newTestRouter(t)
	_, userTk := seedUser(t, db, "u@x.com", models.RoleUser)
	_, adminTk := seedUser(t, db, "a@x.com", models.RoleAdmin)
	if rec := request(t, r, http.MethodGet, "/api/v1/users/me", userTk, nil); rec.Code != http.StatusOK {
		t.Errorf("me expected 200, got %d", rec.Code)
	}
	if rec := request(t, r, http.MethodGet, "/api/v1/users", userTk, nil); rec.Code != http.StatusForbidden {
		t.Errorf("role guard expected 403, got %d", rec.Code)
	}
	if rec := request(t, r, http.MethodGet, "/api/v1/users", adminTk, nil); rec.Code != http.StatusOK {
		t.Errorf("admin list expected 200, got %d", rec.Code)
	}
}
