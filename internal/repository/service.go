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

const serviceColumns = "id, business_id, category_id, name, description, min_price, max_price, duration, advance_payment_amount, is_extra, is_active, created_at, updated_at"

func scanService(row pgx.Row) (*model.Service, error) {
	var s model.Service
	if err := row.Scan(
		&s.ID, &s.BusinessID, &s.CategoryID, &s.Name, &s.Description,
		&s.MinPrice, &s.MaxPrice, &s.Duration, &s.AdvancePaymentAmount,
		&s.IsExtra, &s.IsActive, &s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scanning service: %w", err)
	}
	return &s, nil
}

func (r *serviceRepo) collectServices(rows pgx.Rows) ([]model.Service, error) {
	defer rows.Close()
	out := make([]model.Service, 0)
	for rows.Next() {
		s, err := scanService(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating services: %w", err)
	}
	return out, nil
}

func (r *serviceRepo) List(ctx context.Context, businessID uuid.UUID) ([]model.Service, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+serviceColumns+` FROM services WHERE business_id = $1 ORDER BY created_at`,
		businessID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing services: %w", err)
	}
	return r.collectServices(rows)
}

func (r *serviceRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Service, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+serviceColumns+` FROM services WHERE id = $1`, id)
	return scanService(row)
}

func (r *serviceRepo) Create(ctx context.Context, s *model.Service) error {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO services
		    (business_id, category_id, name, description, min_price, max_price,
		     duration, advance_payment_amount, is_extra, is_active)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id, created_at, updated_at`,
		s.BusinessID, s.CategoryID, s.Name, s.Description, s.MinPrice, s.MaxPrice,
		s.Duration, s.AdvancePaymentAmount, s.IsExtra, s.IsActive,
	)
	if err := row.Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return fmt.Errorf("inserting service: %w", err)
	}
	return nil
}

func (r *serviceRepo) Update(ctx context.Context, id uuid.UUID, s *model.Service) error {
	cmd, err := r.pool.Exec(ctx,
		`UPDATE services
		    SET category_id = $1, name = $2, description = $3, min_price = $4,
		        max_price = $5, duration = $6, advance_payment_amount = $7,
		        is_extra = $8, is_active = $9, updated_at = NOW()
		  WHERE id = $10`,
		s.CategoryID, s.Name, s.Description, s.MinPrice, s.MaxPrice, s.Duration,
		s.AdvancePaymentAmount, s.IsExtra, s.IsActive, id,
	)
	if err != nil {
		return fmt.Errorf("updating service: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *serviceRepo) Delete(ctx context.Context, id uuid.UUID) error {
	cmd, err := r.pool.Exec(ctx, `DELETE FROM services WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting service: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *serviceRepo) ListExtras(ctx context.Context, serviceID uuid.UUID) ([]model.Service, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+prefixedServiceColumns("e")+`
		   FROM service_extras se
		   JOIN services e ON e.id = se.extra_id
		  WHERE se.service_id = $1
		  ORDER BY e.created_at`,
		serviceID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing service extras: %w", err)
	}
	return r.collectServices(rows)
}

func (r *serviceRepo) LinkExtra(ctx context.Context, serviceID, extraID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO service_extras (service_id, extra_id) VALUES ($1, $2)
		 ON CONFLICT (service_id, extra_id) DO NOTHING`,
		serviceID, extraID,
	)
	if err != nil {
		return fmt.Errorf("linking service extra: %w", err)
	}
	return nil
}

func (r *serviceRepo) UnlinkExtra(ctx context.Context, serviceID, extraID uuid.UUID) error {
	cmd, err := r.pool.Exec(ctx,
		`DELETE FROM service_extras WHERE service_id = $1 AND extra_id = $2`,
		serviceID, extraID,
	)
	if err != nil {
		return fmt.Errorf("unlinking service extra: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *serviceRepo) ListByBusinessURL(ctx context.Context, url string) ([]model.Service, error) {
	return nil, errors.New("not implemented")
}

// prefixedServiceColumns returns the service column list aliased to alias.
// Used when joining services into itself (extras lookup).
func prefixedServiceColumns(alias string) string {
	cols := []string{
		"id", "business_id", "category_id", "name", "description", "min_price",
		"max_price", "duration", "advance_payment_amount", "is_extra", "is_active",
		"created_at", "updated_at",
	}
	out := ""
	for i, c := range cols {
		if i > 0 {
			out += ", "
		}
		out += alias + "." + c
	}
	return out
}
