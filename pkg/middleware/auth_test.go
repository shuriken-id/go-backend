package middleware

import (
	"net/http"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v3"
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

func app(chain ...fiber.Handler) *fiber.App {
	r := fiber.New()
	handlers := make([]any, len(chain))
	for i, h := range chain {
		handlers[i] = h
	}
	r.Get("/", handlers[0], handlers[1:]...)
	return r
}

func doRequest(t *testing.T, a *fiber.App, tokenStr string) int {
	t.Helper()
	return doRequestWithHeader(t, a, "Bearer "+tokenStr)
}

func doRequestWithHeader(t *testing.T, a *fiber.App, headerVal string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if headerVal != "" {
		req.Header.Set("Authorization", headerVal)
	}
	resp, err := a.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

func TestRequireAuthValidToken(t *testing.T) {
	db := newTestDB(t)
	u := createUser(t, db, "a@x.com", models.RoleUser)
	tk, err := token.Generate(u.ID, u.Role, testSecret, 24)
	if err != nil {
		t.Fatal(err)
	}
	var sawEmail string
	inner := func(c fiber.Ctx) error {
		sawEmail = CurrentUser(c).Email
		return c.SendStatus(http.StatusOK)
	}
	if code := doRequest(t, app(RequireAuth(db, testSecret), inner), tk); code != http.StatusOK {
		t.Errorf("expected 200, got %d", code)
	}
	if sawEmail != u.Email {
		t.Errorf("expected current user %q, got %q", u.Email, sawEmail)
	}
}

func TestRequireAuthRawTokenWithoutBearerPrefix(t *testing.T) {
	db := newTestDB(t)
	u := createUser(t, db, "raw@x.com", models.RoleUser)
	tk, err := token.Generate(u.ID, u.Role, testSecret, 24)
	if err != nil {
		t.Fatal(err)
	}
	var sawEmail string
	inner := func(c fiber.Ctx) error {
		sawEmail = CurrentUser(c).Email
		return c.SendStatus(http.StatusOK)
	}
	if code := doRequestWithHeader(t, app(RequireAuth(db, testSecret), inner), tk); code != http.StatusOK {
		t.Errorf("expected 200, got %d", code)
	}
	if sawEmail != u.Email {
		t.Errorf("expected current user %q, got %q", u.Email, sawEmail)
	}
}

func TestRequireAuthMissingHeader(t *testing.T) {
	db := newTestDB(t)
	ok := func(c fiber.Ctx) error { return c.SendStatus(http.StatusOK) }
	if code := doRequestWithHeader(t, app(RequireAuth(db, testSecret), ok), ""); code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", code)
	}
}

func TestRequireAuthInvalidToken(t *testing.T) {
	db := newTestDB(t)
	u := createUser(t, db, "a@x.com", models.RoleUser)
	tk, _ := token.Generate(u.ID, u.Role, "wrong-secret", 24)
	ok := func(c fiber.Ctx) error { return c.SendStatus(http.StatusOK) }
	if code := doRequest(t, app(RequireAuth(db, testSecret), ok), tk); code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", code)
	}
}

func TestRequireAuthUserNotInDB(t *testing.T) {
	db := newTestDB(t)
	tk, err := token.Generate(999, models.RoleUser, testSecret, 24)
	if err != nil {
		t.Fatal(err)
	}
	ok := func(c fiber.Ctx) error { return c.SendStatus(http.StatusOK) }
	if code := doRequest(t, app(RequireAuth(db, testSecret), ok), tk); code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", code)
	}
}

func TestRequireRole(t *testing.T) {
	db := newTestDB(t)
	user := createUser(t, db, "u@x.com", models.RoleUser)
	admin := createUser(t, db, "a@x.com", models.RoleAdmin)
	ok := func(c fiber.Ctx) error { return c.SendStatus(http.StatusOK) }
	adminRoute := app(RequireAuth(db, testSecret), RequireRole(models.RoleAdmin), ok)

	userTk, _ := token.Generate(user.ID, user.Role, testSecret, 24)
	if code := doRequest(t, adminRoute, userTk); code != http.StatusForbidden {
		t.Errorf("expected 403 for user, got %d", code)
	}
	adminTk, _ := token.Generate(admin.ID, admin.Role, testSecret, 24)
	if code := doRequest(t, adminRoute, adminTk); code != http.StatusOK {
		t.Errorf("expected 200 for admin, got %d", code)
	}
}
