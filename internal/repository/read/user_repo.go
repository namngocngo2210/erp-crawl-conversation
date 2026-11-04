package read

import (
	"context"

	"github.com/desi/crawl-conversations/internal/entity"
	"github.com/desi/crawl-conversations/internal/repository"
	"gorm.io/gorm/clause"
)

type UserRepository interface {
	FindByEmail(ctx context.Context, email string) (entity.User, error)
	CreateIfNotExists(ctx context.Context, u entity.User) error
}

type userRepo struct{ db *repository.DB }

func NewUserRepository(db *repository.DB) UserRepository { return &userRepo{db} }

func (r *userRepo) FindByEmail(ctx context.Context, email string) (entity.User, error) {
	var u entity.User
	err := r.db.Gorm.WithContext(ctx).Where("email = ?", email).First(&u).Error
	return u, err
}

func (r *userRepo) CreateIfNotExists(ctx context.Context, u entity.User) error {
	return r.db.Gorm.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "email"}},
		DoNothing: true,
	}).Create(&u).Error
}
