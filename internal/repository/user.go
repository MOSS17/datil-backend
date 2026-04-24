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

var ErrNotFound = errors.New("not found")

type UserRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByBusinessID(ctx context.Context, businessID uuid.UUID) (*model.User, error)
	Create(ctx context.Context, tx pgx.Tx, u *model.User) error
	Update(ctx context.Context, id uuid.UUID, u *model.User) error
}

type userRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &userRepo{pool: pool}
}

const userColumns = "id, business_id, name, email, password, created_at, updated_at"

func scanUser(row pgx.Row) (*model.User, error) {
	var u model.User
	if err := row.Scan(&u.ID, &u.BusinessID, &u.Name, &u.Email, &u.Password, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scanning user: %w", err)
	}
	return &u, nil
}

func (r *userRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id)
	return scanUser(row)
}

func (r *userRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE email = $1`, email)
	return scanUser(row)
}

// GetByBusinessID returns the (currently single) user that owns a business.
// The booking flow uses this to attribute reserved appointments to the
// owner — appointments.user_id is the FK, not business_id. If multiple users
// per business is added later, this needs a smarter selection rule.
func (r *userRepo) GetByBusinessID(ctx context.Context, businessID uuid.UUID) (*model.User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE business_id = $1 ORDER BY created_at LIMIT 1`,
		businessID,
	)
	return scanUser(row)
}

func (r *userRepo) Create(ctx context.Context, tx pgx.Tx, u *model.User) error {
	row := tx.QueryRow(ctx,
		`INSERT INTO users (business_id, name, email, password)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at, updated_at`,
		u.BusinessID, u.Name, u.Email, u.Password,
	)
	if err := row.Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return fmt.Errorf("inserting user: %w", err)
	}
	return nil
}

func (r *userRepo) Update(ctx context.Context, id uuid.UUID, u *model.User) error {
	cmd, err := r.pool.Exec(ctx,
		`UPDATE users SET name = $1, email = $2, updated_at = NOW() WHERE id = $3`,
		u.Name, u.Email, id,
	)
	if err != nil {
		return fmt.Errorf("updating user: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
