package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mossandoval/datil-api/internal/model"
)

type BusinessRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Business, error)
	GetByURL(ctx context.Context, url string) (*model.Business, error)
	Create(ctx context.Context, tx pgx.Tx, b *model.Business) error
	Update(ctx context.Context, id uuid.UUID, b *model.Business) error
	UpdateBank(ctx context.Context, id uuid.UUID, clabe, bankName, beneficiaryName string) error
	UpdateLogo(ctx context.Context, id uuid.UUID, logoURL string) error
}

type businessRepo struct {
	pool *pgxpool.Pool
}

func NewBusinessRepository(pool *pgxpool.Pool) BusinessRepository {
	return &businessRepo{pool: pool}
}

func (r *businessRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Business, error) {
	return nil, errors.New("not implemented")
}

func (r *businessRepo) GetByURL(ctx context.Context, url string) (*model.Business, error) {
	return nil, errors.New("not implemented")
}

func (r *businessRepo) Create(ctx context.Context, tx pgx.Tx, b *model.Business) error {
	return errors.New("not implemented")
}

func (r *businessRepo) Update(ctx context.Context, id uuid.UUID, b *model.Business) error {
	return errors.New("not implemented")
}

func (r *businessRepo) UpdateBank(ctx context.Context, id uuid.UUID, clabe, bankName, beneficiaryName string) error {
	return errors.New("not implemented")
}

func (r *businessRepo) UpdateLogo(ctx context.Context, id uuid.UUID, logoURL string) error {
	return errors.New("not implemented")
}
