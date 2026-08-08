package handlers

import (
	"github.com/gofiber/fiber/v3"

	"go-backend/internal/dto"
)

func respondError(c fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(dto.ErrorResponse{Error: message})
}
