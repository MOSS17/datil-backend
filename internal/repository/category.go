package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mossandoval/datil-api/internal/model"
)

type CategoryRepository interface {
	List(ctx context.Context, businessID uuid.UUID) ([]model.Category, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Category, error)
	Create(ctx context.Context, c *model.Category) error
	Update(ctx context.Context, id uuid.UUID, c *model.Category) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type categoryRepo struct {
	pool *pgxpool.Pool
}

func NewCategoryRepository(pool *pgxpool.Pool) CategoryRepository {
	return &categoryRepo{pool: pool}
}

func (r *categoryRepo) List(ctx context.Context, businessID uuid.UUID) ([]model.Category, error) {
	return nil, errors.New("not implemented")
}

func (r *categoryRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Category, error) {
	return nil, errors.New("not implemented")
}

func (r *categoryRepo) Create(ctx context.Context, c *model.Category) error {
	return errors.New("not implemented")
}

func (r *categoryRepo) Update(ctx context.Context, id uuid.UUID, c *model.Category) error {
	return errors.New("not implemented")
}

func (r *categoryRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return errors.New("not implemented")
}
