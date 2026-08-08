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
