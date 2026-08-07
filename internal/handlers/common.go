package handlers

import (
	"github.com/gin-gonic/gin"

	"go-backend/internal/dto"
)

func respondError(c *gin.Context, status int, message string) {
	c.JSON(status, dto.ErrorResponse{Error: message})
}