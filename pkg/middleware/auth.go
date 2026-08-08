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
		header := strings.TrimSpace(c.Get("Authorization"))
		if header == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(dto.ErrorResponse{Error: "missing or invalid authorization header"})
		}
		tokenStr := header
		if strings.HasPrefix(strings.ToLower(header), "bearer ") {
			tokenStr = strings.TrimSpace(header[7:])
		}
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
