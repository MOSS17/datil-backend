package repository

import (
	"context"
	"errors"
	"fmt"

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

const businessColumns = "id, name, location, description, logo_url, url, beneficiary_clabe, bank_name, beneficiary_name, created_at, updated_at"

func scanBusiness(row pgx.Row) (*model.Business, error) {
	var b model.Business
	if err := row.Scan(
		&b.ID, &b.Name, &b.Location, &b.Description, &b.LogoURL, &b.URL,
		&b.BeneficiaryClabe, &b.BankName, &b.BeneficiaryName, &b.CreatedAt, &b.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scanning business: %w", err)
	}
	return &b, nil
}

func (r *businessRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Business, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+businessColumns+` FROM businesses WHERE id = $1`, id)
	return scanBusiness(row)
}

func (r *businessRepo) GetByURL(ctx context.Context, url string) (*model.Business, error) {
	return nil, errors.New("not implemented")
}

func (r *businessRepo) Create(ctx context.Context, tx pgx.Tx, b *model.Business) error {
	row := tx.QueryRow(ctx,
		`INSERT INTO businesses (name, url)
		 VALUES ($1, $2)
		 RETURNING id, created_at, updated_at`,
		b.Name, b.URL,
	)
	if err := row.Scan(&b.ID, &b.CreatedAt, &b.UpdatedAt); err != nil {
		return fmt.Errorf("inserting business: %w", err)
	}
	return nil
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
