package main

import (
	"log"
	"os"

	"github.com/gofiber/contrib/v3/swaggerui"
	"github.com/joho/godotenv"

	"go-backend/internal/router"
	"go-backend/pkg/config"
	"go-backend/pkg/database"
)

// @title       Go REST API Template API
// @version     1.0
// @description A template REST API with JWT auth and todo CRUD.
// @BasePath    /api/v1

// @securityDefinitions.apikey BearerAuth
// @in   header
// @name Authorization
// @description Type "Bearer " followed by token or enter token directly.

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
