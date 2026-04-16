package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mossandoval/datil-api/internal/model"
)

type ScheduleRepository interface {
	ListWorkdays(ctx context.Context, businessID uuid.UUID) ([]model.Workday, error)
	UpsertWorkdays(ctx context.Context, businessID uuid.UUID, workdays []model.Workday) error
	ListPersonalTime(ctx context.Context, userID uuid.UUID) ([]model.PersonalTime, error)
	CreatePersonalTime(ctx context.Context, pt *model.PersonalTime) error
	DeletePersonalTime(ctx context.Context, id uuid.UUID) error
}

type scheduleRepo struct {
	pool *pgxpool.Pool
}

func NewScheduleRepository(pool *pgxpool.Pool) ScheduleRepository {
	return &scheduleRepo{pool: pool}
}

func (r *scheduleRepo) ListWorkdays(ctx context.Context, businessID uuid.UUID) ([]model.Workday, error) {
	return nil, errors.New("not implemented")
}

func (r *scheduleRepo) UpsertWorkdays(ctx context.Context, businessID uuid.UUID, workdays []model.Workday) error {
	return errors.New("not implemented")
}

func (r *scheduleRepo) ListPersonalTime(ctx context.Context, userID uuid.UUID) ([]model.PersonalTime, error) {
	return nil, errors.New("not implemented")
}

func (r *scheduleRepo) CreatePersonalTime(ctx context.Context, pt *model.PersonalTime) error {
	return errors.New("not implemented")
}

func (r *scheduleRepo) DeletePersonalTime(ctx context.Context, id uuid.UUID) error {
	return errors.New("not implemented")
}
