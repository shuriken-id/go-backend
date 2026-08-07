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
