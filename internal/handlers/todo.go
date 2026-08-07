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