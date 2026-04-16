package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mossandoval/datil-api/internal/model"
)

type UserRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	Create(ctx context.Context, tx pgx.Tx, u *model.User) error
	Update(ctx context.Context, id uuid.UUID, u *model.User) error
}

type userRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &userRepo{pool: pool}
}

func (r *userRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	return nil, errors.New("not implemented")
}

func (r *userRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	return nil, errors.New("not implemented")
}

func (r *userRepo) Create(ctx context.Context, tx pgx.Tx, u *model.User) error {
	return errors.New("not implemented")
}

func (r *userRepo) Update(ctx context.Context, id uuid.UUID, u *model.User) error {
	return errors.New("not implemented")
}
