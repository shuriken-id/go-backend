# Fiber v3 Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Gin HTTP layer of the Go REST API template with Fiber v3 (fiber v3.4.0) + `gofiber/contrib/v3/swaggerui`, keeping all routes, auth semantics, models, services, and tests passing.

**Architecture:** Framework-agnostic layers (`internal/models`, `internal/dto` shapes, `internal/services`, `pkg/token`, `pkg/database`) stay unchanged. The HTTP layer (`pkg/middleware`, `internal/handlers`, `internal/router`, `main.go`) is rewritten to Fiber v3's API: handlers are `func(c fiber.Ctx) error`, binding uses `c.Bind().Body()`, path params use `fiber.Params[int]`. Swagger UI moves to `gofiber/contrib/v3/swaggerui` which serves `docs/swagger.json` from disk (no runtime swag dependency), so `docs/docs.go` is deleted from the module and gitignored.

**Tech Stack:** Fiber v3 (`github.com/gofiber/fiber/v3`), `github.com/gofiber/contrib/v3/swaggerui`, `github.com/go-playground/validator/v10`, GORM, golang-jwt/v5, sqlite (test-only).

## Global Constraints

- Module name stays `go-backend`; Go `1.26.4`.
- Handlers take `c fiber.Ctx` (interface, NOT `*fiber.Ctx`) — Fiber v3 style.
- DTO tags change from `binding:` to `validate:` (Fiber convention). Same rules: `required`, `email`, `min=6`.
- `docs/docs.go` is REMOVED from the module and added to `.gitignore`. Only `docs/swagger.json` and `docs/swagger.yaml` are committed. `main.go` must NOT import `_ "go-backend/docs"`.
- No comments in code except swag `@` annotation blocks (existing project convention).
- Final gate: `go test ./... -race -count=1` passes.
- Swagger UI path: `/swagger` (BasePath `/`, Path `swagger`), title `Go REST API Template API`.
- Config field `GinMode` renamed to `AppEnv`; env var `GIN_MODE` renamed to `APP_ENV`.
- DB is an empty Neon Postgres — AutoMigrate from scratch is safe.

---

### Task 1: Swap dependencies in go.mod

**Files:**
- Modify: `go.mod`, `go.sum` (via `go get` / `go mod tidy`)
- Modify: `.env.example` (rename GIN_MODE → APP_ENV)

**Interfaces:**
- Consumes: nothing.
- Produces: `github.com/gofiber/fiber/v3`, `github.com/gofiber/contrib/v3/swaggerui`, `github.com/go-playground/validator/v10` available; gin removed. Downstream tasks depend on these being in `go.mod`.

- [x] **Step 1: Add Fiber deps**

Run (in repo root):

```bash
go get github.com/gofiber/fiber/v3@v3.4.0
go get github.com/gofiber/contrib/v3/swaggerui@latest
go get github.com/go-playground/validator/v10@latest
```

Do NOT run `go mod tidy` here: nothing imports the new modules yet, so tidy would drop them. Tidy runs in Task 5 after all gin imports are gone.

Expected: `go.mod` gains `github.com/gofiber/fiber/v3`, `github.com/gofiber/contrib/v3/swaggerui`, `github.com/go-playground/validator/v10` in `require`. Gin deps remain for now (still imported by handlers/router/middleware) and are dropped in Task 5's tidy.

- [x] **Step 2: Update .env.example**

Edit `.env.example`: replace `GIN_MODE=debug` with `APP_ENV=development`.

- [x] **Step 3: Commit**

```bash
git add go.mod go.sum .env.example
git commit -m "build: swap gin for fiber v3 and swaggerui deps"
```

---

### Task 2: Config AppEnv + DTO validation tags

**Files:**
- Modify: `pkg/config/config.go`
- Modify: `internal/dto/auth.go`, `internal/dto/todo.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `config.Config` with field `AppEnv string` (no `GinMode`). DTOs tagged with `validate:` instead of `binding:`. Handlers (Task 4) and router (Task 5) rely on `cfg.AppEnv` and the validate tags.

- [x] **Step 1: Rename GinMode → AppEnv in config**

In `pkg/config/config.go`, change:

```go
GinMode    string
```
to:
```go
AppEnv     string
```

and in `Load()` change `GinMode: getenv("GIN_MODE", "debug")` to `AppEnv: getenv("APP_ENV", "development")`.

- [x] **Step 2: Migrate DTO tags**

In `internal/dto/auth.go`, change all `binding:` tags to `validate:`:

```go
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}
```

In `internal/dto/todo.go`, change `binding:"required"` to `validate:"required"` on `CreateTodoRequest.Title`.

- [x] **Step 3: Verify build + commit**

Run: `go build ./pkg/config/... ./internal/dto/...`
Expected: compiles (these packages don't import gin).

```bash
git add pkg/config/config.go internal/dto/auth.go internal/dto/todo.go
git commit -m "refactor: rename GinMode to AppEnv, migrate dto tags to validate"
```

---

### Task 3: Rewrite middleware for Fiber v3

**Files:**
- Modify: `pkg/middleware/auth.go`, `pkg/middleware/cors.go`
- Modify: `pkg/middleware/auth_test.go`
- Test: `pkg/middleware/auth_test.go`

**Interfaces:**
- Consumes: `token.Parse(tokenStr, secret) (*token.Claims, error)`; `models.User`, `models.RoleAdmin`; `dto.ErrorResponse`.
- Produces:
  - `RequireAuth(db *gorm.DB, secret string) fiber.Handler` — returns 401 with `{"error": msg}` on missing/invalid token or unknown user; stores `*models.User` via `c.Locals(userKey, &user)`; calls `c.Next()`.
  - `RequireRole(role string) fiber.Handler` — 403 `{"error":"insufficient permissions"}` if role mismatch.
  - `CurrentUser(c fiber.Ctx) *models.User` — reads from `c.Locals(userKey)`.
  - `CORS() fiber.Handler`.
  - Router (Task 5) and handlers (Task 4) use these exact signatures.

- [x] **Step 1: Rewrite auth.go**

Replace `pkg/middleware/auth.go` with:

```go
package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"go-backend/internal/dto"
	"go-backend/internal/models"
	"go-backend/pkg/token"
)

const userKey = "user"

func RequireAuth(db *gorm.DB, secret string) fiber.Handler {
	return func(c fiber.Ctx) error {
		header := c.Get("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(dto.ErrorResponse{Error: "missing or invalid authorization header"})
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims, err := token.Parse(tokenStr, secret)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(dto.ErrorResponse{Error: err.Error()})
		}
		var user models.User
		if err := db.First(&user, claims.Subject).Error; err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(dto.ErrorResponse{Error: "user not found"})
		}
		c.Locals(userKey, &user)
		return c.Next()
	}
}

func RequireRole(role string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if CurrentUser(c).Role != role {
			return c.Status(fiber.StatusForbidden).JSON(dto.ErrorResponse{Error: "insufficient permissions"})
		}
		return c.Next()
	}
}

func CurrentUser(c fiber.Ctx) *models.User {
	return c.Locals(userKey).(*models.User)
}
```

- [x] **Step 2: Rewrite cors.go**

Replace `pkg/middleware/cors.go` with:

```go
package middleware

import "github.com/gofiber/fiber/v3"

func CORS() fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Method() == fiber.MethodOptions {
			return c.SendStatus(fiber.StatusNoContent)
		}
		return c.Next()
	}
}
```

- [x] **Step 3: Rewrite auth_test.go**

Replace `pkg/middleware/auth_test.go` with:

```go
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
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if tokenStr != "" {
		req.Header.Set("Authorization", "Bearer "+tokenStr)
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

func TestRequireAuthMissingHeader(t *testing.T) {
	db := newTestDB(t)
	ok := func(c fiber.Ctx) error { return c.SendStatus(http.StatusOK) }
	if code := doRequest(t, app(RequireAuth(db, testSecret), ok), ""); code != http.StatusUnauthorized {
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
```

- [x] **Step 4: Run middleware tests**

Run: `go test ./pkg/middleware/... -count=1`
Expected: PASS (5 tests). Note: `go build ./...` at the repo level may fail until Task 5 — that is expected; only `pkg/middleware` must compile and pass here.

- [x] **Step 5: Commit**

```bash
git add pkg/middleware/auth.go pkg/middleware/cors.go pkg/middleware/auth_test.go
git commit -m "feat: rewrite auth and cors middleware for fiber v3"
```

---

### Task 4: Rewrite handlers for Fiber v3

**Files:**
- Modify: `internal/handlers/common.go`, `internal/handlers/auth.go`, `internal/handlers/todo.go`, `internal/handlers/user.go`
- Modify: `internal/handlers/handlers_test.go`
- Test: `internal/handlers/handlers_test.go`

**Interfaces:**
- Consumes: `middleware.CurrentUser(c fiber.Ctx) *models.User`; `services.AuthService.Register/Login`, `services.TodoService.{ListByOwner,Create,Get,Update,Delete}`, `services.UserService.{Me?no,List}`; `services.ErrEmailTaken`, `services.ErrInvalidCredentials`, `services.ErrNotFound`; `dto.{RegisterRequest,LoginRequest,LoginResponse,UserResponse,CreateTodoRequest,UpdateTodoRequest,TodoResponse,ErrorResponse,FromUser,FromTodo}`; `models.RoleAdmin`.
- Produces: handler methods with signature `func(c fiber.Ctx) error` (e.g. `(*AuthHandler).Register`, `(*TodoHandler).List`, `(*UserHandler).Me`). Router (Task 5) registers these by name.

- [x] **Step 1: Rewrite common.go**

Replace `internal/handlers/common.go` with:

```go
package handlers

import (
	"github.com/gofiber/fiber/v3"

	"go-backend/internal/dto"
)

func respondError(c fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(dto.ErrorResponse{Error: message})
}
```

- [x] **Step 2: Rewrite auth.go**

Replace `internal/handlers/auth.go` with (keep the swag `@` annotation blocks exactly as they are now — only the body changes):

```go
package handlers

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"go-backend/internal/dto"
	"go-backend/internal/services"
)

type AuthHandler struct {
	svc *services.AuthService
}

func NewAuthHandler(svc *services.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// Register godoc
// @Summary     Register a new user
// @Description Creates a user account with the "user" role and returns it.
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       body body dto.RegisterRequest true "Register payload"
// @Success     201 {object} dto.UserResponse
// @Failure     400 {object} dto.ErrorResponse
// @Failure     409 {object} dto.ErrorResponse
// @Router      /api/v1/auth/register [post]
func (h *AuthHandler) Register(c fiber.Ctx) error {
	var req dto.RegisterRequest
	if err := c.Bind().Body(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "invalid request body")
	}
	user, err := h.svc.Register(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, services.ErrEmailTaken) {
			return respondError(c, http.StatusConflict, err.Error())
		}
		return respondError(c, http.StatusInternalServerError, "failed to register")
	}
	return c.Status(http.StatusCreated).JSON(dto.FromUser(*user))
}

// Login godoc
// @Summary     Login and get a JWT
// @Description Verifies an existing user and returns a bearer token.
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       body body dto.LoginRequest true "Login payload"
// @Success     200 {object} dto.LoginResponse
// @Failure     400 {object} dto.ErrorResponse
// @Failure     401 {object} dto.ErrorResponse
// @Router      /api/v1/auth/login [post]
func (h *AuthHandler) Login(c fiber.Ctx) error {
	var req dto.LoginRequest
	if err := c.Bind().Body(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "invalid request body")
	}
	tk, err := h.svc.Login(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			return respondError(c, http.StatusUnauthorized, err.Error())
		}
		return respondError(c, http.StatusInternalServerError, "failed to login")
	}
	return c.Status(http.StatusOK).JSON(dto.LoginResponse{Token: tk})
}
```

- [x] **Step 3: Rewrite todo.go**

Replace `internal/handlers/todo.go` with (keep all swag annotations):

```go
package handlers

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"go-backend/internal/dto"
	"go-backend/internal/models"
	"go-backend/internal/services"
	"go-backend/pkg/middleware"
)

type TodoHandler struct {
	svc *services.TodoService
}

func NewTodoHandler(svc *services.TodoService) *TodoHandler {
	return &TodoHandler{svc: svc}
}

// List godoc
// @Summary     List current user's todos
// @Tags        todos
// @Produce     json
// @Security    BearerAuth
// @Success     200 {array} dto.TodoResponse
// @Failure     401 {object} dto.ErrorResponse
// @Router      /api/v1/todos [get]
func (h *TodoHandler) List(c fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	todos, err := h.svc.ListByOwner(user.ID)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to list todos")
	}
	resp := make([]dto.TodoResponse, 0, len(todos))
	for _, t := range todos {
		resp = append(resp, dto.FromTodo(t))
	}
	return c.JSON(resp)
}

// Create godoc
// @Summary     Create a todo for the current user
// @Tags        todos
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body dto.CreateTodoRequest true "Todo payload"
// @Success     201 {object} dto.TodoResponse
// @Failure     400 {object} dto.ErrorResponse
// @Failure     401 {object} dto.ErrorResponse
// @Router      /api/v1/todos [post]
func (h *TodoHandler) Create(c fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	var req dto.CreateTodoRequest
	if err := c.Bind().Body(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "title is required")
	}
	todo, err := h.svc.Create(user.ID, req.Title, req.Description)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to create todo")
	}
	return c.Status(http.StatusCreated).JSON(dto.FromTodo(*todo))
}

// Get godoc
// @Summary     Get a single todo
// @Tags        todos
// @Produce     json
// @Security    BearerAuth
// @Param       id path int true "Todo ID"
// @Success     200 {object} dto.TodoResponse
// @Failure     401 {object} dto.ErrorResponse
// @Failure     404 {object} dto.ErrorResponse
// @Router      /api/v1/todos/{id} [get]
func (h *TodoHandler) Get(c fiber.Ctx) error {
	id, ok := parseID(c)
	if !ok {
		return nil
	}
	user := middleware.CurrentUser(c)
	todo, err := h.svc.Get(id, user.ID, user.Role == models.RoleAdmin)
	if err != nil {
		return respondTodoError(c, err)
	}
	return c.JSON(dto.FromTodo(*todo))
}

// Update godoc
// @Summary     Update a todo
// @Tags        todos
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id   path int                  true "Todo ID"
// @Param       body body dto.UpdateTodoRequest true "Fields to update"
// @Success     200  {object} dto.TodoResponse
// @Failure     400  {object} dto.ErrorResponse
// @Failure     401  {object} dto.ErrorResponse
// @Failure     404  {object} dto.ErrorResponse
// @Router      /api/v1/todos/{id} [put]
func (h *TodoHandler) Update(c fiber.Ctx) error {
	id, ok := parseID(c)
	if !ok {
		return nil
	}
	user := middleware.CurrentUser(c)
	var req dto.UpdateTodoRequest
	if err := c.Bind().Body(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "invalid request body")
	}
	updates := make(map[string]interface{})
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Done != nil {
		updates["done"] = *req.Done
	}
	todo, err := h.svc.Update(id, user.ID, user.Role == models.RoleAdmin, updates)
	if err != nil {
		return respondTodoError(c, err)
	}
	return c.JSON(dto.FromTodo(*todo))
}

// Delete godoc
// @Summary     Delete a todo
// @Tags        todos
// @Security    BearerAuth
// @Param       id path int true "Todo ID"
// @Success     204 "no content"
// @Failure     401 {object} dto.ErrorResponse
// @Failure     404 {object} dto.ErrorResponse
// @Router      /api/v1/todos/{id} [delete]
func (h *TodoHandler) Delete(c fiber.Ctx) error {
	id, ok := parseID(c)
	if !ok {
		return nil
	}
	user := middleware.CurrentUser(c)
	if err := h.svc.Delete(id, user.ID, user.Role == models.RoleAdmin); err != nil {
		return respondTodoError(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

func parseID(c fiber.Ctx) (uint, bool) {
	id := fiber.Params[int](c, "id", 0)
	if id <= 0 {
		respondError(c, http.StatusBadRequest, "invalid todo id")
		return 0, false
	}
	return uint(id), true
}

func respondTodoError(c fiber.Ctx, err error) error {
	if errors.Is(err, services.ErrNotFound) {
		return respondError(c, http.StatusNotFound, err.Error())
	}
	return respondError(c, http.StatusInternalServerError, "todo operation failed")
}
```

- [x] **Step 4: Rewrite user.go**

Replace `internal/handlers/user.go` with (keep swag annotations):

```go
package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"go-backend/internal/dto"
	"go-backend/internal/services"
	"go-backend/pkg/middleware"
)

type UserHandler struct {
	svc *services.UserService
}

func NewUserHandler(svc *services.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// Me godoc
// @Summary     Get current user profile
// @Tags        users
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} dto.UserResponse
// @Failure     401 {object} dto.ErrorResponse
// @Router      /api/v1/users/me [get]
func (h *UserHandler) Me(c fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	return c.JSON(dto.FromUser(*user))
}

// List godoc
// @Summary     List all users (admin only)
// @Tags        users
// @Produce     json
// @Security    BearerAuth
// @Success     200 {array} dto.UserResponse
// @Failure     401 {object} dto.ErrorResponse
// @Failure     403 {object} dto.ErrorResponse
// @Router      /api/v1/users [get]
func (h *UserHandler) List(c fiber.Ctx) error {
	users, err := h.svc.List()
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to list users")
	}
	resp := make([]dto.UserResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, dto.FromUser(u))
	}
	return c.JSON(resp)
}
```

- [x] **Step 5: Rewrite handlers_test.go**

Replace `internal/handlers/handlers_test.go` with:

```go
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
```

Note: `newTestRouter` calls `router.New`, which is rewritten in Task 5 — this task's test cannot compile until Task 5 lands. That is expected; run the full suite in Task 5 Step 3.

- [x] **Step 6: Commit handler code (without passing tests yet)**

```bash
git add internal/handlers/common.go internal/handlers/auth.go internal/handlers/todo.go internal/handlers/user.go internal/handlers/handlers_test.go
git commit -m "feat: rewrite handlers for fiber v3"
```

---

### Task 5: Rewrite router + main + register swaggerui

**Files:**
- Modify: `internal/router/router.go`
- Modify: `main.go`
- Modify: `docs/` (regenerate; delete `docs/docs.go`)
- Modify: `.gitignore`
- Test: full suite `go test ./... -race -count=1`

**Interfaces:**
- Consumes: `handlers.{NewAuthHandler,NewTodoHandler,NewUserHandler}` with Fiber methods; `middleware.{RequireAuth,RequireRole,CORS}`; `services.{NewAuthService,NewTodoService,NewUserService}`; `config.Config` (now has `AppEnv`); `models.RoleAdmin`.
- Produces: `router.New(db *gorm.DB, cfg *config.Config) *fiber.App`; `main.go` boots Fiber, mounts swaggerui, serves `:PORT`.

- [x] **Step 1: Rewrite router.go**

Replace `internal/router/router.go` with:

```go
package router

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"go-backend/internal/handlers"
	"go-backend/internal/models"
	"go-backend/internal/services"
	"go-backend/pkg/config"
	"go-backend/pkg/middleware"
)

type structValidator struct {
	validate *validator.Validate
}

func (v *structValidator) Validate(out any) error {
	return v.validate.Struct(out)
}

func New(db *gorm.DB, cfg *config.Config) *fiber.App {
	app := fiber.New(fiber.Config{
		StructValidator: &structValidator{validate: validator.New()},
	})
	app.Use(middleware.CORS())

	authSvc := services.NewAuthService(db, cfg.JWTSecret, cfg.TokenHours)
	todoSvc := services.NewTodoService(db)
	userSvc := services.NewUserService(db)

	authH := handlers.NewAuthHandler(authSvc)
	todoH := handlers.NewTodoHandler(todoSvc)
	userH := handlers.NewUserHandler(userSvc)

	app.Get("/healthz", func(c fiber.Ctx) error {
		return c.JSON(map[string]string{"status": "ok"})
	})

	api := app.Group("/api/v1")
	{
		auth := api.Group("/auth")
		auth.Post("/register", authH.Register)
		auth.Post("/login", authH.Login)

		protected := api.Group("")
		protected.Use(middleware.RequireAuth(db, cfg.JWTSecret))
		protected.Get("/users/me", userH.Me)
		protected.Get("/todos", todoH.List)
		protected.Post("/todos", todoH.Create)
		protected.Get("/todos/:id", todoH.Get)
		protected.Put("/todos/:id", todoH.Update)
		protected.Delete("/todos/:id", todoH.Delete)

		admin := protected.Group("/users")
		admin.Use(middleware.RequireRole(models.RoleAdmin))
		admin.Get("/", userH.List)
	}

	return app
}
```

- [x] **Step 2: Rewrite main.go**

Replace `main.go` with:

```go
package main

import (
	"log"
	"os"

	"github.com/gofiber/contrib/v3/swaggerui"
	"github.com/gofiber/fiber/v3"
	"github.com/joho/godotenv"

	"go-backend/internal/router"
	"go-backend/pkg/config"
	"go-backend/pkg/database"
)

// @title       Go REST API Template API
// @version     1.0
// @description A template REST API with JWT auth and todo CRUD.
// @host        localhost:8080
// @BasePath    /api/v1

// @securityDefinitions.apikey BearerAuth
// @in   header
// @name Authorization

func main() {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		log.Fatalf("failed to load .env: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := database.Connect(cfg.DBURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	app := router.New(db, cfg)
	app.Use(swaggerui.New(swaggerui.Config{
		BasePath: "/",
		FilePath: "./docs/swagger.json",
		Path:     "swagger",
		Title:    "Go REST API Template API",
	}))
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
```

- [x] **Step 3: Regenerate swagger docs, delete docs.go, update .gitignore**

Run:

```bash
swag init
rm docs/docs.go
```

Add `docs/docs.go` to `.gitignore` (append to the existing file).

Verify `docs/swagger.json` and `docs/swagger.yaml` exist and `docs/docs.go` does not.

- [x] **Step 4: Run full test suite**

Run: `go mod tidy && go test ./... -race -count=1`
Expected: ALL tests PASS (services, middleware, handlers). If `go mod tidy` complains about missing gin packages, first `go mod tidy` may need `-e`; the clean sequence is: tidy, then `go test ./... -race -count=1`.

- [x] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: wire fiber router, main, and swaggerui; drop docs.go"
```

---

### Task 6: Verify module count reduction + README

**Files:**
- Modify: `README.md`

**Interfaces:**
- Consumes: final `go.mod`.
- Produces: accurate README.

- [x] **Step 1: Confirm module count**

Run: `go list -m all | wc -l`
Expected: ~77 (down from 114 with Gin). If it shows ~98, `docs/docs.go` exists or is still imported — remove it and re-run `go mod tidy`.

- [x] **Step 2: Update README**

Update `README.md`:
- Replace Gin references with Fiber v3.
- Tech stack list: Fiber v3, Swag + `gofiber/contrib/v3/swaggerui`, GORM, JWT, bcrypt.
- Swagger URL stays `http://localhost:8080/swagger`.
- Config table: replace `GIN_MODE` with `APP_ENV` (default `development`).
- Regenerate docs step: add `rm docs/docs.go` after `swag init` (keep only swagger.json/yaml committed).
- Note that swagger UI serves `docs/swagger.json` from disk.

- [x] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: update README for fiber v3"
```

---

### Task 7: Live smoke test against Neon

**Files:** none (runtime verification).

**Interfaces:**
- Consumes: final binary + `.env`.
- Produces: verified boot, health, swagger, register.

- [x] **Step 1: Boot the server**

Run: `go run .` (with `.env` pointing at the empty Neon DB).
Expected: server starts on `:8080`, AutoMigrate creates `users` and `todos` tables.

- [x] **Step 2: Smoke-test endpoints**

Run:

```bash
curl -s http://localhost:8080/healthz          # expect {"status":"ok"}
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/swagger   # expect 200
curl -s -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"smoke@example.com","password":"password123"}'           # expect 201 + JSON user
```

- [x] **Step 3: Stop the server and commit any fixes**

Kill the server (`Ctrl+C`). If any endpoint misbehaves, fix in the relevant file and rerun `go test ./... -race -count=1` before committing.

```bash
git add -A && git commit -m "fix: smoke test adjustments"
```

(Skip commit if no changes were needed.)
