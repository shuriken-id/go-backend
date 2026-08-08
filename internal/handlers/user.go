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
// @Param       Authorization header string false "Bearer token atau token JWT"
// @Success     200 {object} dto.UserResponse
// @Failure     401 {object} dto.ErrorResponse
// @Router      /users/me [get]
func (h *UserHandler) Me(c fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	return c.JSON(dto.FromUser(*user))
}

// List godoc
// @Summary     List all users (admin only)
// @Tags        users
// @Produce     json
// @Security    BearerAuth
// @Param       Authorization header string false "Bearer token atau token JWT"
// @Success     200 {array} dto.UserResponse
// @Failure     401 {object} dto.ErrorResponse
// @Failure     403 {object} dto.ErrorResponse
// @Router      /users [get]
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
