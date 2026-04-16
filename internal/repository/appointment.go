package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mossandoval/datil-api/internal/model"
)

type AppointmentRepository interface {
	List(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.Appointment, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Appointment, error)
	Create(ctx context.Context, a *model.Appointment, services []model.AppointmentService) error
	Update(ctx context.Context, id uuid.UUID, a *model.Appointment) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByDateRange(ctx context.Context, businessID uuid.UUID, from, to time.Time) ([]model.Appointment, error)
}

type appointmentRepo struct {
	pool *pgxpool.Pool
}

func NewAppointmentRepository(pool *pgxpool.Pool) AppointmentRepository {
	return &appointmentRepo{pool: pool}
}

func (r *appointmentRepo) List(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.Appointment, error) {
	return nil, errors.New("not implemented")
}

func (r *appointmentRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Appointment, error) {
	return nil, errors.New("not implemented")
}

func (r *appointmentRepo) Create(ctx context.Context, a *model.Appointment, services []model.AppointmentService) error {
	return errors.New("not implemented")
}

func (r *appointmentRepo) Update(ctx context.Context, id uuid.UUID, a *model.Appointment) error {
	return errors.New("not implemented")
}

func (r *appointmentRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return errors.New("not implemented")
}

func (r *appointmentRepo) ListByDateRange(ctx context.Context, businessID uuid.UUID, from, to time.Time) ([]model.Appointment, error) {
	return nil, errors.New("not implemented")
}
