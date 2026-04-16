package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mossandoval/datil-api/internal/model"
)

type DashboardRepository interface {
	GetDashboard(ctx context.Context, userID, businessID uuid.UUID) (*model.DashboardData, error)
}

type dashboardRepo struct {
	pool *pgxpool.Pool
}

func NewDashboardRepository(pool *pgxpool.Pool) DashboardRepository {
	return &dashboardRepo{pool: pool}
}

func (r *dashboardRepo) GetDashboard(ctx context.Context, userID, businessID uuid.UUID) (*model.DashboardData, error) {
	return nil, errors.New("not implemented")
}
