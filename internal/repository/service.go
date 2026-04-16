package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mossandoval/datil-api/internal/model"
)

type ServiceRepository interface {
	List(ctx context.Context, businessID uuid.UUID) ([]model.Service, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Service, error)
	Create(ctx context.Context, s *model.Service) error
	Update(ctx context.Context, id uuid.UUID, s *model.Service) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListExtras(ctx context.Context, serviceID uuid.UUID) ([]model.Service, error)
	LinkExtra(ctx context.Context, serviceID, extraID uuid.UUID) error
	UnlinkExtra(ctx context.Context, serviceID, extraID uuid.UUID) error
	ListByBusinessURL(ctx context.Context, url string) ([]model.Service, error)
}

type serviceRepo struct {
	pool *pgxpool.Pool
}

func NewServiceRepository(pool *pgxpool.Pool) ServiceRepository {
	return &serviceRepo{pool: pool}
}

func (r *serviceRepo) List(ctx context.Context, businessID uuid.UUID) ([]model.Service, error) {
	return nil, errors.New("not implemented")
}

func (r *serviceRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Service, error) {
	return nil, errors.New("not implemented")
}

func (r *serviceRepo) Create(ctx context.Context, s *model.Service) error {
	return errors.New("not implemented")
}

func (r *serviceRepo) Update(ctx context.Context, id uuid.UUID, s *model.Service) error {
	return errors.New("not implemented")
}

func (r *serviceRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return errors.New("not implemented")
}

func (r *serviceRepo) ListExtras(ctx context.Context, serviceID uuid.UUID) ([]model.Service, error) {
	return nil, errors.New("not implemented")
}

func (r *serviceRepo) LinkExtra(ctx context.Context, serviceID, extraID uuid.UUID) error {
	return errors.New("not implemented")
}

func (r *serviceRepo) UnlinkExtra(ctx context.Context, serviceID, extraID uuid.UUID) error {
	return errors.New("not implemented")
}

func (r *serviceRepo) ListByBusinessURL(ctx context.Context, url string) ([]model.Service, error) {
	return nil, errors.New("not implemented")
}
