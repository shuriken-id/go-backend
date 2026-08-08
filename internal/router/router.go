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
