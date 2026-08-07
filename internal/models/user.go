package models

import "time"

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

type User struct {
	ID           uint      `gorm:"primaryKey"`
	Email        string    `gorm:"uniqueIndex;not null"`
	PasswordHash string    `gorm:"not null"`
	Role         string    `gorm:"not null;default:user"`
	Todos        []Todo    `gorm:"constraint:OnDelete:CASCADE;foreignKey:OwnerID;references:ID"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
