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

type CalendarRepository interface {
	GetByUserAndProvider(ctx context.Context, userID uuid.UUID, provider string) (*model.CalendarIntegration, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.CalendarIntegration, error)
	Upsert(ctx context.Context, ci *model.CalendarIntegration) error
	Delete(ctx context.Context, userID uuid.UUID, provider string) error
}

type calendarRepo struct {
	pool *pgxpool.Pool
}

func NewCalendarRepository(pool *pgxpool.Pool) CalendarRepository {
	return &calendarRepo{pool: pool}
}

const calendarIntegrationColumns = "id, user_id, provider, access_token, refresh_token, account_email, is_active, expires_at, created_at, updated_at"

func scanCalendarIntegration(row pgx.Row) (*model.CalendarIntegration, error) {
	var ci model.CalendarIntegration
	if err := row.Scan(
		&ci.ID, &ci.UserID, &ci.Provider, &ci.AccessToken, &ci.RefreshToken, &ci.AccountEmail,
		&ci.IsActive, &ci.ExpiresAt, &ci.CreatedAt, &ci.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scanning calendar integration: %w", err)
	}
	return &ci, nil
}

func (r *calendarRepo) GetByUserAndProvider(ctx context.Context, userID uuid.UUID, provider string) (*model.CalendarIntegration, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+calendarIntegrationColumns+`
		   FROM calendar_integrations
		  WHERE user_id = $1 AND provider = $2 AND is_active = true`,
		userID, provider,
	)
	return scanCalendarIntegration(row)
}

func (r *calendarRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.CalendarIntegration, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+calendarIntegrationColumns+`
		   FROM calendar_integrations
		  WHERE user_id = $1 AND is_active = true
		  ORDER BY provider`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing calendar integrations: %w", err)
	}
	defer rows.Close()
	out := make([]model.CalendarIntegration, 0)
	for rows.Next() {
		ci, err := scanCalendarIntegration(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *ci)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating calendar integrations: %w", err)
	}
	return out, nil
}

// Upsert inserts a new integration or refreshes tokens on an existing
// (user_id, provider) pair. The UNIQUE(user_id, provider) constraint on the
// table drives the ON CONFLICT branch. Called from two paths: the OAuth
// exchange handler (initial connect) and the Google token-refresh goroutine
// in the reserve push (rotated tokens must land back in the DB).
func (r *calendarRepo) Upsert(ctx context.Context, ci *model.CalendarIntegration) error {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO calendar_integrations
		    (user_id, provider, access_token, refresh_token, account_email, is_active, expires_at)
		 VALUES ($1, $2, $3, $4, $5, true, $6)
		 ON CONFLICT (user_id, provider) DO UPDATE
		    SET access_token  = EXCLUDED.access_token,
		        refresh_token = COALESCE(EXCLUDED.refresh_token, calendar_integrations.refresh_token),
		        account_email = COALESCE(EXCLUDED.account_email, calendar_integrations.account_email),
		        is_active     = true,
		        expires_at    = EXCLUDED.expires_at,
		        updated_at    = NOW()
		 RETURNING id, created_at, updated_at`,
		ci.UserID, ci.Provider, ci.AccessToken, ci.RefreshToken, ci.AccountEmail, ci.ExpiresAt,
	)
	if err := row.Scan(&ci.ID, &ci.CreatedAt, &ci.UpdatedAt); err != nil {
		return fmt.Errorf("upserting calendar integration: %w", err)
	}
	ci.IsActive = true
	return nil
}

func (r *calendarRepo) Delete(ctx context.Context, userID uuid.UUID, provider string) error {
	cmd, err := r.pool.Exec(ctx,
		`DELETE FROM calendar_integrations WHERE user_id = $1 AND provider = $2`,
		userID, provider,
	)
	if err != nil {
		return fmt.Errorf("deleting calendar integration: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
