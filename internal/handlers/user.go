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
