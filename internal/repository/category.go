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

const categoryColumns = "id, business_id, name, allow_multiple, created_at, updated_at"

func scanCategory(row pgx.Row) (*model.Category, error) {
	var c model.Category
	if err := row.Scan(&c.ID, &c.BusinessID, &c.Name, &c.AllowMultiple, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scanning category: %w", err)
	}
	return &c, nil
}

func (r *categoryRepo) List(ctx context.Context, businessID uuid.UUID) ([]model.Category, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+categoryColumns+` FROM categories WHERE business_id = $1 ORDER BY created_at`,
		businessID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing categories: %w", err)
	}
	defer rows.Close()

	out := make([]model.Category, 0)
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating categories: %w", err)
	}
	return out, nil
}

func (r *categoryRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Category, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+categoryColumns+` FROM categories WHERE id = $1`, id)
	return scanCategory(row)
}

func (r *categoryRepo) Create(ctx context.Context, c *model.Category) error {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO categories (business_id, name, allow_multiple)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at, updated_at`,
		c.BusinessID, c.Name, c.AllowMultiple,
	)
	if err := row.Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return fmt.Errorf("inserting category: %w", err)
	}
	return nil
}

func (r *categoryRepo) Update(ctx context.Context, id uuid.UUID, c *model.Category) error {
	cmd, err := r.pool.Exec(ctx,
		`UPDATE categories SET name = $1, allow_multiple = $2, updated_at = NOW() WHERE id = $3`,
		c.Name, c.AllowMultiple, id,
	)
	if err != nil {
		return fmt.Errorf("updating category: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *categoryRepo) Delete(ctx context.Context, id uuid.UUID) error {
	cmd, err := r.pool.Exec(ctx, `DELETE FROM categories WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting category: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
