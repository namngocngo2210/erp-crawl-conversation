package entity

import "time"

type User struct {
	ID           int64 `gorm:"primaryKey"`
	Email        string
	PasswordHash string
	Name         string
	Role         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (User) TableName() string { return "users" }
