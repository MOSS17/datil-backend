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

const businessColumns = "id, name, location, description, logo_url, url, timezone, beneficiary_clabe, bank_name, beneficiary_name, created_at, updated_at"

// DefaultBusinessTimezone is used when signup omits one — the MVP is
// Mexico-targeted so it's the safe fallback. Migration 000016 set the same
// default at the DB level as a belt-and-suspenders.
const DefaultBusinessTimezone = "America/Mexico_City"

func scanBusiness(row pgx.Row) (*model.Business, error) {
	var b model.Business
	if err := row.Scan(
		&b.ID, &b.Name, &b.Location, &b.Description, &b.LogoURL, &b.URL, &b.Timezone,
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
	row := r.pool.QueryRow(ctx, `SELECT `+businessColumns+` FROM businesses WHERE url = $1`, url)
	return scanBusiness(row)
}

func (r *businessRepo) Create(ctx context.Context, tx pgx.Tx, b *model.Business) error {
	if b.Timezone == "" {
		b.Timezone = DefaultBusinessTimezone
	}
	row := tx.QueryRow(ctx,
		`INSERT INTO businesses (name, url, timezone)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at, updated_at`,
		b.Name, b.URL, b.Timezone,
	)
	if err := row.Scan(&b.ID, &b.CreatedAt, &b.UpdatedAt); err != nil {
		return fmt.Errorf("inserting business: %w", err)
	}
	return nil
}

func (r *businessRepo) Update(ctx context.Context, id uuid.UUID, b *model.Business) error {
	cmd, err := r.pool.Exec(ctx,
		`UPDATE businesses
		    SET name = $1, location = $2, description = $3, updated_at = NOW()
		  WHERE id = $4`,
		b.Name, b.Location, b.Description, id,
	)
	if err != nil {
		return fmt.Errorf("updating business: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *businessRepo) UpdateBank(ctx context.Context, id uuid.UUID, clabe, bankName, beneficiaryName string) error {
	cmd, err := r.pool.Exec(ctx,
		`UPDATE businesses
		    SET beneficiary_clabe = $1, bank_name = $2, beneficiary_name = $3, updated_at = NOW()
		  WHERE id = $4`,
		clabe, bankName, beneficiaryName, id,
	)
	if err != nil {
		return fmt.Errorf("updating business bank: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *businessRepo) UpdateLogo(ctx context.Context, id uuid.UUID, logoURL string) error {
	cmd, err := r.pool.Exec(ctx,
		`UPDATE businesses SET logo_url = $1, updated_at = NOW() WHERE id = $2`,
		logoURL, id,
	)
	if err != nil {
		return fmt.Errorf("updating business logo: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
