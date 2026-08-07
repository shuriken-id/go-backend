package models

import "time"

type Todo struct {
	ID          uint      `gorm:"primaryKey"`
	Title       string    `gorm:"not null"`
	Description string
	Done        bool      `gorm:"not null;default:false"`
	OwnerID     uint      `gorm:"not null;index"`
	Owner       User      `json:"-"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
