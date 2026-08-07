package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"go-backend/internal/models"
	"go-backend/pkg/token"
)

const testSecret = "test-secret"

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Todo{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func createUser(t *testing.T, db *gorm.DB, email, role string) *models.User {
	t.Helper()
	u := &models.User{Email: email, PasswordHash: "h", Role: role}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func app(chain ...gin.HandlerFunc) http.Handler {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/", chain...)
	return r
}

func doRequest(t *testing.T, h http.Handler, tokenStr string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if tokenStr != "" {
		req.Header.Set("Authorization", "Bearer "+tokenStr)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRequireAuthValidToken(t *testing.T) {
	db := newTestDB(t)
	u := createUser(t, db, "a@x.com", models.RoleUser)
	tk, err := token.Generate(u.ID, u.Role, testSecret, 24)
	if err != nil {
		t.Fatal(err)
	}
	var sawEmail string
	inner := func(c *gin.Context) { sawEmail = CurrentUser(c).Email; c.Status(http.StatusOK) }
	if rec := doRequest(t, app(RequireAuth(db, testSecret), inner), tk); rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if sawEmail != u.Email {
		t.Errorf("expected current user %q, got %q", u.Email, sawEmail)
	}
}

func TestRequireAuthMissingHeader(t *testing.T) {
	db := newTestDB(t)
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	if rec := doRequest(t, app(RequireAuth(db, testSecret), ok), ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAuthInvalidToken(t *testing.T) {
	db := newTestDB(t)
	u := createUser(t, db, "a@x.com", models.RoleUser)
	tk, _ := token.Generate(u.ID, u.Role, "wrong-secret", 24)
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	if rec := doRequest(t, app(RequireAuth(db, testSecret), ok), tk); rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAuthUserNotInDB(t *testing.T) {
	db := newTestDB(t)
	tk, err := token.Generate(999, models.RoleUser, testSecret, 24)
	if err != nil {
		t.Fatal(err)
	}
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	if rec := doRequest(t, app(RequireAuth(db, testSecret), ok), tk); rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRequireRole(t *testing.T) {
	db := newTestDB(t)
	user := createUser(t, db, "u@x.com", models.RoleUser)
	admin := createUser(t, db, "a@x.com", models.RoleAdmin)
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	adminRoute := app(RequireAuth(db, testSecret), RequireRole(models.RoleAdmin), ok)

	userTk, _ := token.Generate(user.ID, user.Role, testSecret, 24)
	if rec := doRequest(t, adminRoute, userTk); rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for user, got %d", rec.Code)
	}
	adminTk, _ := token.Generate(admin.ID, admin.Role, testSecret, 24)
	if rec := doRequest(t, adminRoute, adminTk); rec.Code != http.StatusOK {
		t.Errorf("expected 200 for admin, got %d", rec.Code)
	}
}
