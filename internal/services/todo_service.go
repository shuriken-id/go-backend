package services

import (
	"errors"

	"gorm.io/gorm"

	"go-backend/internal/models"
)

type TodoService struct {
	db *gorm.DB
}

func NewTodoService(db *gorm.DB) *TodoService {
	return &TodoService{db: db}
}

func (s *TodoService) Create(ownerID uint, title, description string) (*models.Todo, error) {
	todo := &models.Todo{Title: title, Description: description, OwnerID: ownerID}
	if err := s.db.Create(todo).Error; err != nil {
		return nil, err
	}
	return todo, nil
}

func (s *TodoService) ListByOwner(ownerID uint) ([]models.Todo, error) {
	var todos []models.Todo
	if err := s.db.Where("owner_id = ?", ownerID).Order("created_at desc").Find(&todos).Error; err != nil {
		return nil, err
	}
	return todos, nil
}

func (s *TodoService) Get(id, userID uint, isAdmin bool) (*models.Todo, error) {
	var todo models.Todo
	if err := s.db.Where("id = ? AND (owner_id = ? OR ?)", id, userID, isAdmin).First(&todo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &todo, nil
}

func (s *TodoService) Update(id, userID uint, isAdmin bool, updates map[string]interface{}) (*models.Todo, error) {
	var todo models.Todo
	if err := s.db.Where("id = ? AND (owner_id = ? OR ?)", id, userID, isAdmin).First(&todo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err := s.db.Model(&todo).Updates(updates).Error; err != nil {
		return nil, err
	}
	return &todo, nil
}

func (s *TodoService) Delete(id, userID uint, isAdmin bool) error {
	res := s.db.Where("id = ? AND (owner_id = ? OR ?)", id, userID, isAdmin).Delete(&models.Todo{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
