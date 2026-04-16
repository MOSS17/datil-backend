package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mossandoval/datil-api/internal/model"
)

type CalendarRepository interface {
	GetByUserAndProvider(ctx context.Context, userID uuid.UUID, provider string) (*model.CalendarIntegration, error)
	Upsert(ctx context.Context, ci *model.CalendarIntegration) error
	Delete(ctx context.Context, userID uuid.UUID, provider string) error
}

type calendarRepo struct {
	pool *pgxpool.Pool
}

func NewCalendarRepository(pool *pgxpool.Pool) CalendarRepository {
	return &calendarRepo{pool: pool}
}

func (r *calendarRepo) GetByUserAndProvider(ctx context.Context, userID uuid.UUID, provider string) (*model.CalendarIntegration, error) {
	return nil, errors.New("not implemented")
}

func (r *calendarRepo) Upsert(ctx context.Context, ci *model.CalendarIntegration) error {
	return errors.New("not implemented")
}

func (r *calendarRepo) Delete(ctx context.Context, userID uuid.UUID, provider string) error {
	return errors.New("not implemented")
}
