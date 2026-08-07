package dto

import (
	"time"

	"go-backend/internal/models"
)

type CreateTodoRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
}

type UpdateTodoRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Done        *bool   `json:"done"`
}

type TodoResponse struct {
	ID          uint      `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Done        bool      `json:"done"`
	OwnerID     uint      `json:"owner_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func FromTodo(t models.Todo) TodoResponse {
	return TodoResponse{
		ID:          t.ID,
		Title:       t.Title,
		Description: t.Description,
		Done:        t.Done,
		OwnerID:     t.OwnerID,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}