# Go REST API Template Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a reusable Go REST API template (Gin + Swag + GORM + JWT auth with roles) backed by Postgres, with one sample protected Todo CRUD resource.

**Architecture:** Layered — `pkg/` holds infrastructure (config, database, middleware, token), `internal/` holds application code (models, dto, services, handlers, router). Handlers parse HTTP and delegate to services, which own all DB access via GORM. All routes live under `/api/v1`; docs mount at `/swagger/index.html`.

**Tech Stack:** Go 1.26 · gin-gonic/gin · swaggo/swag + swaggo/gin-swagger + swaggo/files · gorm.io/gorm + gorm.io/driver/postgres · golang-jwt/jwt/v5 · golang.org/x/crypto bcrypt · joho/godotenv · glebarez/sqlite (pure-Go, only for tests)

## Global Constraints

- Module name is exactly `go-backend`.
- Go version 1.26 (already installed).
- No explanatory code comments except the swag `@` annotation blocks, which are required for doc generation.
- All bodies are JSON. Error bodies are always `{"error": "<message>"}`.
- Protected routes require `Authorization: Bearer <jwt>`.
- Tests must run without a real DB: in-memory SQLite via `github.com/glebarez/sqlite` (pure Go, no CGO).
- Config keys from env/`.env`: `PORT`, `POSTGRESQL_DATABASE`, `JWT_SECRET`, `TOKEN_HOURS`, `GIN_MODE`.
- The project is not yet a git repo; initialize it in Task 1.

---

### Task 1: Scaffold module, git, config

**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Create: `.env.example`
- Modify: `.env` (existing file, holds `POSTGRESQL_DATABASE`)
- Create: `pkg/config/config.go`

**Interfaces:**
- Produces: `config.Config` struct and `func Load() (*Config, error)`.

- [ ] **Step 1: Initialize module and git**

```bash
cd /home/rosemary/Documents/go-backend
go mod init go-backend
git init
```

- [ ] **Step 2: Create `.gitignore`**

```gitignore
.env
*.exe
bin/
```

- [ ] **Step 3: Create `.env.example`**

```env
PORT=8080
POSTGRESQL_DATABASE=postgresql://user:password@host:5432/dbname?sslmode=require
JWT_SECRET=change-me-to-a-long-random-string
TOKEN_HOURS=24
GIN_MODE=debug
```

Append `JWT_SECRET`, `PORT`, and `TOKEN_HOURS` to the existing `.env` so it can run (the Neon URL is already present as `POSTGRESQL_DATABASE`).

- [ ] **Step 4: Create `pkg/config/config.go`**

```go
package config

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	Port       string
	DBURL      string
	JWTSecret  string
	TokenHours int
	GinMode    string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:       getenv("PORT", "8080"),
		DBURL:      os.Getenv("POSTGRESQL_DATABASE"),
		JWTSecret:  os.Getenv("JWT_SECRET"),
		TokenHours: getenvInt("TOKEN_HOURS", 24),
		GinMode:    getenv("GIN_MODE", "debug"),
	}
	if cfg.DBURL == "" {
		return nil, errors.New("POSTGRESQL_DATABASE is required")
	}
	if cfg.JWTSecret == "" {
		return nil, errors.New("JWT_SECRET is required")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
```

- [ ] **Step 5: Build and commit**

```bash
go build ./...
git add go.mod .gitignore .env.example pkg/config
git commit -m "chore: scaffold module, gitignore, and config package"
```

Expected: `go: downloading ...` then success, then a commit.

---

### Task 2: Models, token package, database

**Files:**
- Create: `internal/models/user.go`
- Create: `internal/models/todo.go`
- Create: `pkg/token/token.go`
- Create: `pkg/database/db.go`

**Interfaces:**
- Produces:
  - `models.User`: `ID uint`, `Email string`, `PasswordHash string`, `Role string`, `Todos []Todo`, `CreatedAt`, `UpdatedAt`.
  - `models.Todo`: `ID uint`, `Title string`, `Description string`, `Done bool`, `OwnerID uint`, `CreatedAt`, `UpdatedAt`.
  - Constants `models.RoleUser = "user"`, `models.RoleAdmin = "admin"`.
  - `token.Generate(userID uint, role, secret string, hours int) (string, error)`.
  - `token.Parse(tokenStr, secret string) (*token.Claims, error)`; `Claims` has `Role string` and embeds `jwt.RegisteredClaims`.
  - `database.Connect(dsn string) (*gorm.DB, error)`, `database.AutoMigrate(db *gorm.DB) error`.

- [ ] **Step 1: Create `internal/models/user.go`**

```go
package models

import "time"

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

type User struct {
	ID           uint      `gorm:"primaryKey"`
	Email        string    `gorm:"uniqueIndex;not null"`
	PasswordHash string    `gorm:"not null"`
	Role         string    `gorm:"not null;default:user"`
	Todos        []Todo    `gorm:"constraint:OnDelete:CASCADE"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
```

- [ ] **Step 2: Create `internal/models/todo.go`**

```go
package models

import "time"

type Todo struct {
	ID          uint      `gorm:"primaryKey"`
	Title       string    `gorm:"not null"`
	Description string
	Done        bool      `gorm:"not null;default:false"`
	OwnerID     uint      `gorm:"not null;index"`
	Owner       User      `json:"-"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
```

- [ ] **Step 3: Create `pkg/token/token.go`**

```go
package token

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func Generate(userID uint, role, secret string, hours int) (string, error) {
	claims := Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(hours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(secret))
}

func Parse(tokenStr, secret string) (*Claims, error) {
	claims := &Claims{}
	t, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !t.Valid {
		return nil, errors.New("invalid or expired token")
	}
	return claims, nil
}
```

- [ ] **Step 4: Create `pkg/database/db.go`**

```go
package database

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"go-backend/internal/models"
)

func Connect(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&models.User{}, &models.Todo{})
}
```

- [ ] **Step 5: Fetch deps, build, commit**

```bash
go get gorm.io/gorm gorm.io/driver/postgres github.com/golang-jwt/jwt/v5
go build ./...
git add internal/models pkg/token pkg/database
git commit -m "feat: add models, jwt token helper, and database bootstrap"
```

---

### Task 3: DTOs

**Files:**
- Create: `internal/dto/common.go`
- Create: `internal/dto/auth.go`
- Create: `internal/dto/todo.go`

**Interfaces:**
- Consumes: `models.User`, `models.Todo`.
- Produces:
  - `dto.ErrorResponse{ Error string }`.
  - `dto.RegisterRequest{ Email; Password string }`, `dto.LoginRequest{ Email; Password string }`, `dto.LoginResponse{ Token string }`.
  - `dto.UserResponse{ ID uint; Email; Role string; CreatedAt; UpdatedAt time.Time }` + `func FromUser(models.User) dto.UserResponse`.
  - `dto.CreateTodoRequest{ Title; Description string }`, `dto.UpdateTodoRequest{ Title *string; Description *string; Done *bool }`, `dto.TodoResponse{...}` + `func FromTodo(models.Todo) dto.TodoResponse`.

- [ ] **Step 1: Create `internal/dto/common.go`**

```go
package dto

type ErrorResponse struct {
	Error string `json:"error"`
}
```

- [ ] **Step 2: Create `internal/dto/auth.go`**

```go
package dto

import (
	"time"

	"go-backend/internal/models"
)

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type UserResponse struct {
	ID        uint      `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func FromUser(u models.User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}
```

- [ ] **Step 3: Create `internal/dto/todo.go`**

```go
package dto

import (
	"time"

	"go-backend/internal/models"
)

type CreateTodoRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
}

type UpdateTodoRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Done        *bool   `json:"done"`
}

type TodoResponse struct {
	ID          uint      `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Done        bool      `json:"done"`
	OwnerID     uint      `json:"owner_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func FromTodo(t models.Todo) TodoResponse {
	return TodoResponse{
		ID:          t.ID,
		Title:       t.Title,
		Description: t.Description,
		Done:        t.Done,
		OwnerID:     t.OwnerID,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}
```

- [ ] **Step 4: Build and commit**

```bash
go build ./...
git add internal/dto
git commit -m "feat: add request/response dto package"
```

---

### Task 4: Auth service + tests

**Files:**
- Create: `internal/services/errors.go`
- Create: `internal/services/auth_service.go`
- Test: `internal/services/auth_service_test.go`

**Interfaces:**
- Consumes: `models`, `token.Generate`.
- Produces:
  - Sentinel errors `ErrEmailTaken`, `ErrInvalidCredentials`, `ErrNotFound` (package-level vars in `services`).
  - `services.NewAuthService(db *gorm.DB, secret string, tokenHours int) *AuthService`.
  - `(*AuthService).Register(email, password string) (*models.User, error)`.
  - `(*AuthService).Login(email, password string) (string, error)`.
  - `(*AuthService).GenerateToken(user *models.User) (string, error)`.

- [ ] **Step 1: Create `internal/services/errors.go`**

```go
package services

import "errors"

var (
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrNotFound           = errors.New("resource not found")
)
```

- [ ] **Step 2: Write the failing test `internal/services/auth_service_test.go`**

```go
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
```

- [ ] **Step 3: Run test to verify it fails to compile**

```bash
go get github.com/glebarez/sqlite
go test ./internal/services/ -run TestRegisterNormalizesEmail -v
```

Expected: FAIL (compile error: `undefined: NewAuthService`).

- [ ] **Step 4: Implement `internal/services/auth_service.go`**

```go
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
```

- [ ] **Step 5: Fetch bcrypt dep, run tests**

```bash
go get golang.org/x/crypto/bcrypt
go test ./internal/services/
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/services
git commit -m "feat: add auth service with register, login, and jwt issuance"
```

---

### Task 5: Todo and User services + tests

**Files:**
- Create: `internal/services/todo_service.go`
- Create: `internal/services/user_service.go`
- Test: `internal/services/todo_service_test.go`
- Test: `internal/services/user_service_test.go`

**Interfaces:**
- Consumes: `models`, `ErrNotFound`.
- Produces:
  - `TodoService`, `NewTodoService(db *gorm.DB) *TodoService`:
    - `Create(ownerID uint, title, description string) (*models.Todo, error)`
    - `ListByOwner(ownerID uint) ([]models.Todo, error)`
    - `Get(id, userID uint, isAdmin bool) (*models.Todo, error)`
    - `Update(id, userID uint, isAdmin bool, updates map[string]interface{}) (*models.Todo, error)`
    - `Delete(id, userID uint, isAdmin bool) error`
  - `UserService`, `NewUserService(db *gorm.DB) *UserService`:
    - `GetByID(id uint) (*models.User, error)`
    - `List() ([]models.User, error)`

Ownership rule: for `Get`/`Update`/`Delete`, a row is reachable only when the caller owns it OR `isAdmin` is true; otherwise return `ErrNotFound` (404, avoids leaking existence).

- [ ] **Step 1: Write the failing tests `internal/services/todo_service_test.go`**

```go
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
```

- [ ] **Step 2: Create `internal/services/todo_service.go`**

```go
package services

import (
	"errors"

	"gorm.io/gorm"

	"go-backend/internal/models"
)

type TodoService struct {
	db *gorm.DB
}

func NewTodoService(db *gorm.DB) *TodoService {
	return &TodoService{db: db}
}

func (s *TodoService) Create(ownerID uint, title, description string) (*models.Todo, error) {
	todo := &models.Todo{Title: title, Description: description, OwnerID: ownerID}
	if err := s.db.Create(todo).Error; err != nil {
		return nil, err
	}
	return todo, nil
}

func (s *TodoService) ListByOwner(ownerID uint) ([]models.Todo, error) {
	var todos []models.Todo
	if err := s.db.Where("owner_id = ?", ownerID).Order("created_at desc").Find(&todos).Error; err != nil {
		return nil, err
	}
	return todos, nil
}

func (s *TodoService) Get(id, userID uint, isAdmin bool) (*models.Todo, error) {
	var todo models.Todo
	if err := s.db.Where("id = ? AND (owner_id = ? OR ?)", id, userID, isAdmin).First(&todo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &todo, nil
}

func (s *TodoService) Update(id, userID uint, isAdmin bool, updates map[string]interface{}) (*models.Todo, error) {
	var todo models.Todo
	if err := s.db.Where("id = ? AND (owner_id = ? OR ?)", id, userID, isAdmin).First(&todo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err := s.db.Model(&todo).Updates(updates).Error; err != nil {
		return nil, err
	}
	return &todo, nil
}

func (s *TodoService) Delete(id, userID uint, isAdmin bool) error {
	res := s.db.Where("id = ? AND (owner_id = ? OR ?)", id, userID, isAdmin).Delete(&models.Todo{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
```

- [ ] **Step 3: Write the failing test `internal/services/user_service_test.go`**

```go
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
```

- [ ] **Step 4: Implement `internal/services/user_service.go`**

```go
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
```

- [ ] **Step 5: Run the tests**

```bash
go test ./internal/services/
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/services
git commit -m "feat: add todo and user services with ownership checks"
```

---

### Task 6: Middleware (CORS, auth, role) + tests

**Files:**
- Create: `pkg/middleware/cors.go`
- Create: `pkg/middleware/auth.go`
- Test: `pkg/middleware/auth_test.go`

**Interfaces:**
- Consumes: `models`, `token`, `dto.ErrorResponse`.
- Produces:
  - `middleware.CORS() gin.HandlerFunc`.
  - `middleware.RequireAuth(db *gorm.DB, secret string) gin.HandlerFunc` — sets `user` (`*models.User`) in the Gin context on success.
  - `middleware.RequireRole(role string) gin.HandlerFunc`.
  - `middleware.CurrentUser(c *gin.Context) *models.User`.

- [ ] **Step 1: Create `pkg/middleware/cors.go`**

```go
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
```

- [ ] **Step 2: Create `pkg/middleware/auth.go`**

```go
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"go-backend/internal/dto"
	"go-backend/internal/models"
	"go-backend/pkg/token"
)

const userKey = "user"

func RequireAuth(db *gorm.DB, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "missing or invalid authorization header"})
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims, err := token.Parse(tokenStr, secret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: err.Error()})
			return
		}
		var user models.User
		if err := db.First(&user, claims.Subject).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "user not found"})
			return
		}
		c.Set(userKey, &user)
		c.Next()
	}
}

func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if CurrentUser(c).Role != role {
			c.AbortWithStatusJSON(http.StatusForbidden, dto.ErrorResponse{Error: "insufficient permissions"})
			return
		}
		c.Next()
	}
}

func CurrentUser(c *gin.Context) *models.User {
	return c.MustGet(userKey).(*models.User)
}
```

- [ ] **Step 3: Write the tests `pkg/middleware/auth_test.go`**

```go
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
```

- [ ] **Step 4: Fetch dep and run the tests**

```bash
go get github.com/gin-gonic/gin
go test ./pkg/middleware/ -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/middleware
git commit -m "feat: add cors, auth, and role middleware with tests"
```

---

### Task 7: Handlers + router + integration tests

**Files:**
- Create: `internal/handlers/common.go`
- Create: `internal/handlers/auth.go`
- Create: `internal/handlers/todo.go`
- Create: `internal/handlers/user.go`
- Create: `internal/router/router.go`
- Test: `internal/handlers/handlers_test.go`

**Interfaces:**
- Consumes: `models`, `dto`, `services.*`, `middleware.*`, `token.Generate`, `config.Config`.
- Produces:
  - `handlers.NewAuthHandler(*services.AuthService) *AuthHandler`: `.Register`, `.Login`.
  - `handlers.NewTodoHandler(*services.TodoService) *TodoHandler`: `.Create`, `.List`, `.Get`, `.Update`, `.Delete`.
  - `handlers.NewUserHandler(*services.UserService) *UserHandler`: `.Me`, `.List`.
  - `router.New(db *gorm.DB, cfg *config.Config) *gin.Engine`: registers all routes and middleware and `/healthz`. Does NOT mount swagger (main.go does that in Task 8).

- [ ] **Step 1: Create `internal/handlers/common.go`**

```go
package handlers

import (
	"github.com/gin-gonic/gin"

	"go-backend/internal/dto"
)

func respondError(c *gin.Context, status int, message string) {
	c.JSON(status, dto.ErrorResponse{Error: message})
}
```

- [ ] **Step 2: Create `internal/handlers/auth.go`**

```go
package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

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
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}
	user, err := h.svc.Register(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, services.ErrEmailTaken) {
			respondError(c, http.StatusConflict, err.Error())
			return
		}
		respondError(c, http.StatusInternalServerError, "failed to register")
		return
	}
	c.JSON(http.StatusCreated, dto.FromUser(*user))
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
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}
	tk, err := h.svc.Login(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			respondError(c, http.StatusUnauthorized, err.Error())
			return
		}
		respondError(c, http.StatusInternalServerError, "failed to login")
		return
	}
	c.JSON(http.StatusOK, dto.LoginResponse{Token: tk})
}
```

- [ ] **Step 3: Create `internal/handlers/todo.go`**

```go
package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

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
func (h *TodoHandler) List(c *gin.Context) {
	user := middleware.CurrentUser(c)
	todos, err := h.svc.ListByOwner(user.ID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to list todos")
		return
	}
	resp := make([]dto.TodoResponse, 0, len(todos))
	for _, t := range todos {
		resp = append(resp, dto.FromTodo(t))
	}
	c.JSON(http.StatusOK, resp)
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
func (h *TodoHandler) Create(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var req dto.CreateTodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "title is required")
		return
	}
	todo, err := h.svc.Create(user.ID, req.Title, req.Description)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create todo")
		return
	}
	c.JSON(http.StatusCreated, dto.FromTodo(*todo))
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
func (h *TodoHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	user := middleware.CurrentUser(c)
	todo, err := h.svc.Get(id, user.ID, user.Role == models.RoleAdmin)
	if err != nil {
		respondTodoError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.FromTodo(*todo))
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
func (h *TodoHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	user := middleware.CurrentUser(c)
	var req dto.UpdateTodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
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
		respondTodoError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.FromTodo(*todo))
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
func (h *TodoHandler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	user := middleware.CurrentUser(c)
	if err := h.svc.Delete(id, user.ID, user.Role == models.RoleAdmin); err != nil {
		respondTodoError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func parseID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid todo id")
		return 0, false
	}
	return uint(id), true
}

func respondTodoError(c *gin.Context, err error) {
	if errors.Is(err, services.ErrNotFound) {
		respondError(c, http.StatusNotFound, err.Error())
		return
	}
	respondError(c, http.StatusInternalServerError, "todo operation failed")
}
```

- [ ] **Step 4: Create `internal/handlers/user.go`**

```go
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

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
func (h *UserHandler) Me(c *gin.Context) {
	user := middleware.CurrentUser(c)
	c.JSON(http.StatusOK, dto.FromUser(*user))
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
func (h *UserHandler) List(c *gin.Context) {
	users, err := h.svc.List()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to list users")
		return
	}
	resp := make([]dto.UserResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, dto.FromUser(u))
	}
	c.JSON(http.StatusOK, resp)
}
```

- [ ] **Step 5: Create `internal/router/router.go`**

```go
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"go-backend/internal/handlers"
	"go-backend/internal/models"
	"go-backend/internal/services"
	"go-backend/pkg/config"
	"go-backend/pkg/middleware"
)

func New(db *gorm.DB, cfg *config.Config) *gin.Engine {
	r := gin.Default()
	r.Use(middleware.CORS())

	authSvc := services.NewAuthService(db, cfg.JWTSecret, cfg.TokenHours)
	todoSvc := services.NewTodoService(db)
	userSvc := services.NewUserService(db)

	authH := handlers.NewAuthHandler(authSvc)
	todoH := handlers.NewTodoHandler(todoSvc)
	userH := handlers.NewUserHandler(userSvc)

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")
	{
		auth := api.Group("/auth")
		auth.POST("/register", authH.Register)
		auth.POST("/login", authH.Login)

		protected := api.Group("")
		protected.Use(middleware.RequireAuth(db, cfg.JWTSecret))
		protected.GET("/users/me", userH.Me)
		protected.GET("/todos", todoH.List)
		protected.POST("/todos", todoH.Create)
		protected.GET("/todos/:id", todoH.Get)
		protected.PUT("/todos/:id", todoH.Update)
		protected.DELETE("/todos/:id", todoH.Delete)

		admin := protected.Group("/users")
		admin.Use(middleware.RequireRole(models.RoleAdmin))
		admin.GET("", userH.List)
	}

	return r
}
```

- [ ] **Step 6: Write the integration tests `internal/handlers/handlers_test.go`**

```go
package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"go-backend/internal/models"
	"go-backend/internal/router"
	"go-backend/pkg/config"
	"go-backend/pkg/token"
)

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
```

The test uses a small helper `formatID` to avoid importing `strconv` inline. Add this helper to `internal/handlers/handlers_test.go`:

```go
import "strconv"

func formatID(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
```

(Add the `strconv` import to the test file's import block.)

- [ ] **Step 7: Fetch deps, build, run the full suite**

```bash
go get github.com/gin-gonic/gin gorm.io/gorm
go build ./...
go test ./...
```

Expected: build passes; all tests PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/handlers internal/router
git commit -m "feat: add handlers, router, and integration tests"
```

---

### Task 8: main.go + Swagger generation + end-to-end verification

**Files:**
- Create: `main.go`
- Generated: `docs/` (via `swag init`)

**Interfaces:**
- Consumes: `config.Load`, `database.Connect`, `database.AutoMigrate`, `router.New`, and the generated blank-import `go-backend/docs`.

- [ ] **Step 1: Install the swag CLI**

```bash
go install github.com/swaggo/swag/cmd/swag@latest
```

- [ ] **Step 2: Create `main.go`**

```go
package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "go-backend/docs"

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
	gin.SetMode(cfg.GinMode)

	db, err := database.Connect(cfg.DBURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	r := router.New(db, cfg)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
```

- [ ] **Step 3: Fetch deps and generate docs**

```bash
go get github.com/joho/godotenv github.com/swaggo/files github.com/swaggo/gin-swagger
swag init
go build ./...
```

`swag init` scans `main.go` and the handler annotations, generating `docs/` (docs.go, swagger.json, swagger.yaml). Note the `@securityDefinitions.apikey` block must be part of the same comment group as `@title` for swag to apply it — if the generated spec lacks a security scheme, move all `//go` block annotations directly above `func main()`.

- [ ] **Step 4: Verify generation**

```bash
ls docs/
grep -c '"/api/v1' docs/swagger.json
```

Expected: `docs.go swagger.json swagger.yaml` present and the count is non-zero (routes registered in the spec).

- [ ] **Step 5: Run the full test suite with race detector**

```bash
go test ./... -race
```

Expected: all PASS.

- [ ] **Step 6: Boot the server and smoke-test**

```bash
go run .
```

In a second terminal:

```bash
curl -s http://localhost:8080/healthz                       # {"status":"ok"}
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8080/swagger/index.html  # 200
curl -s http://localhost:8080/auth/register -X POST -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","password":"password123"}'
```

Stop the server (Ctrl+C in the first terminal) when done.

- [ ] **Step 7: Commit**

```bash
git add main.go docs
git commit -m "feat: wire up main, generate swagger docs"
```

---

## Self-Review (completed)

- **Spec coverage:**
  - Auth register/login + JWT + roles — Tasks 4, 6.
  - Todo CRUD with owner-or-admin checks — Tasks 5, 7.
  - User `me` + admin list — Tasks 5, 7.
  - Health check — Task 7.
  - Config (env vars) + error handling helper — Tasks 1, 7.
  - GORM `AutoMigrate` — Tasks 2, 8.
  - Swagger docs mount at `/swagger/index.html` — Task 8.
  - In-memory SQLite tests — Tasks 4, 5, 6, 7.
- **No placeholders:** every step contains runnable code; no TBDs.
- **Type consistency:** `token.Generate`/`token.Parse` signatures match middleware usage; `middleware.CurrentUser` returns `*models.User`; `services.New*` constructor signatures match `router.New` and handlers; `router.New(db *gorm.DB, cfg *config.Config)` is used identically in `main.go` and `handlers_test.go`; `dto.FromUser`/`FromTodo` match handler return sites.

## Execution Options

- **Subagent-Driven (recommended):** fresh subagent per task + two-stage review, fast iteration.
- **Inline:** execute tasks in this session with checkpoints.
