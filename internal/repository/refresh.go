package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RefreshTokenRepository interface {
	Insert(ctx context.Context, jti uuid.UUID, userID uuid.UUID, expiresAt time.Time) error
	MarkUsed(ctx context.Context, jti uuid.UUID) (alreadyUsed bool, err error)
	DeleteAllForUser(ctx context.Context, userID uuid.UUID) error
}

type refreshTokenRepo struct {
	pool *pgxpool.Pool
}

func NewRefreshTokenRepository(pool *pgxpool.Pool) RefreshTokenRepository {
	return &refreshTokenRepo{pool: pool}
}

func (r *refreshTokenRepo) Insert(ctx context.Context, jti uuid.UUID, userID uuid.UUID, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO refresh_tokens (jti, user_id, expires_at) VALUES ($1, $2, $3)`,
		jti, userID, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("inserting refresh token: %w", err)
	}
	return nil
}

// MarkUsed atomically claims a refresh token. Returns alreadyUsed=true if the
// token was missing, expired, or previously consumed — that's the theft signal.
func (r *refreshTokenRepo) MarkUsed(ctx context.Context, jti uuid.UUID) (bool, error) {
	var returned uuid.UUID
	err := r.pool.QueryRow(ctx,
		`UPDATE refresh_tokens
		    SET used_at = NOW()
		  WHERE jti = $1 AND used_at IS NULL AND expires_at > NOW()
		  RETURNING jti`,
		jti,
	).Scan(&returned)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return true, nil
		}
		return false, fmt.Errorf("marking refresh token used: %w", err)
	}
	return false, nil
}

func (r *refreshTokenRepo) DeleteAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("deleting refresh tokens for user: %w", err)
	}
	return nil
}
