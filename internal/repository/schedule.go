package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mossandoval/datil-api/internal/model"
)

type ScheduleRepository interface {
	ListWorkdays(ctx context.Context, businessID uuid.UUID) ([]model.Workday, error)
	UpsertWorkdays(ctx context.Context, businessID uuid.UUID, workdays []model.Workday) error
	ListPersonalTime(ctx context.Context, userID uuid.UUID) ([]model.PersonalTime, error)
	ListPersonalTimeOverlapping(ctx context.Context, userID uuid.UUID, date time.Time) ([]model.PersonalTime, error)
	CreatePersonalTime(ctx context.Context, pt *model.PersonalTime) error
	DeletePersonalTime(ctx context.Context, id uuid.UUID) error
}

type scheduleRepo struct {
	pool *pgxpool.Pool
}

func NewScheduleRepository(pool *pgxpool.Pool) ScheduleRepository {
	return &scheduleRepo{pool: pool}
}

// ListWorkdays returns all 7 workdays for a business, each with its WorkHours
// pre-loaded. Disabled days are included so callers can render them.
func (r *scheduleRepo) ListWorkdays(ctx context.Context, businessID uuid.UUID) ([]model.Workday, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT
		    d.id, d.business_id, d.day, d.is_enabled, d.created_at, d.updated_at,
		    h.id, h.day_id, h.start_time::text, h.end_time::text, h.created_at, h.updated_at
		   FROM workdays d
		   LEFT JOIN work_hours h ON h.day_id = d.id
		  WHERE d.business_id = $1
		  ORDER BY d.day, h.start_time`,
		businessID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing workdays: %w", err)
	}
	defer rows.Close()

	byID := make(map[uuid.UUID]*model.Workday)
	order := make([]uuid.UUID, 0, 7)
	for rows.Next() {
		var (
			d   model.Workday
			hID *uuid.UUID
			h   model.WorkHour
			hSt *string
			hEt *string
			hC  *time.Time
			hU  *time.Time
		)
		if err := rows.Scan(
			&d.ID, &d.BusinessID, &d.Day, &d.IsEnabled, &d.CreatedAt, &d.UpdatedAt,
			&hID, &h.DayID, &hSt, &hEt, &hC, &hU,
		); err != nil {
			return nil, fmt.Errorf("scanning workday row: %w", err)
		}
		existing, ok := byID[d.ID]
		if !ok {
			d.Hours = []model.WorkHour{}
			byID[d.ID] = &d
			order = append(order, d.ID)
			existing = byID[d.ID]
		}
		if hID != nil {
			h.ID = *hID
			if hSt != nil {
				h.StartTime = *hSt
			}
			if hEt != nil {
				h.EndTime = *hEt
			}
			if hC != nil {
				h.CreatedAt = *hC
			}
			if hU != nil {
				h.UpdatedAt = *hU
			}
			existing.Hours = append(existing.Hours, h)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating workday rows: %w", err)
	}

	out := make([]model.Workday, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out, nil
}

// UpsertWorkdays replaces a business's entire schedule atomically: inserts
// missing workday rows, updates is_enabled on existing ones, and rewrites
// the work_hours children for each day from scratch. Phase 3 only reads;
// the dashboard "schedule" page is what actually exercises this path.
func (r *scheduleRepo) UpsertWorkdays(ctx context.Context, businessID uuid.UUID, workdays []model.Workday) error {
	return WithTransaction(ctx, r.pool, func(tx pgx.Tx) error {
		for _, d := range workdays {
			var dayID uuid.UUID
			err := tx.QueryRow(ctx,
				`INSERT INTO workdays (business_id, day, is_enabled)
				 VALUES ($1, $2, $3)
				 ON CONFLICT (business_id, day) DO UPDATE SET
				    is_enabled = EXCLUDED.is_enabled,
				    updated_at = NOW()
				 RETURNING id`,
				businessID, d.Day, d.IsEnabled,
			).Scan(&dayID)
			if err != nil {
				return fmt.Errorf("upserting workday: %w", err)
			}

			if _, err := tx.Exec(ctx, `DELETE FROM work_hours WHERE day_id = $1`, dayID); err != nil {
				return fmt.Errorf("clearing work_hours: %w", err)
			}
			for _, h := range d.Hours {
				_, err := tx.Exec(ctx,
					`INSERT INTO work_hours (day_id, start_time, end_time) VALUES ($1, $2::time, $3::time)`,
					dayID, h.StartTime, h.EndTime,
				)
				if err != nil {
					return fmt.Errorf("inserting work_hour: %w", err)
				}
			}
		}
		return nil
	})
}

const personalTimeColumns = `id, user_id, start_date::text, end_date::text,
		to_char(start_time, 'HH24:MI:SS'), to_char(end_time, 'HH24:MI:SS'),
		created_at, updated_at`

func scanPersonalTime(row pgx.Row) (*model.PersonalTime, error) {
	var pt model.PersonalTime
	if err := row.Scan(
		&pt.ID, &pt.UserID, &pt.StartDate, &pt.EndDate,
		&pt.StartTime, &pt.EndTime,
		&pt.CreatedAt, &pt.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scanning personal_time: %w", err)
	}
	return &pt, nil
}

func (r *scheduleRepo) collectPersonalTime(rows pgx.Rows) ([]model.PersonalTime, error) {
	defer rows.Close()
	out := make([]model.PersonalTime, 0)
	for rows.Next() {
		pt, err := scanPersonalTime(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *pt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating personal_time: %w", err)
	}
	return out, nil
}

func (r *scheduleRepo) ListPersonalTime(ctx context.Context, userID uuid.UUID) ([]model.PersonalTime, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+personalTimeColumns+` FROM personal_time WHERE user_id = $1 ORDER BY start_date`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing personal_time: %w", err)
	}
	return r.collectPersonalTime(rows)
}

// ListPersonalTimeOverlapping returns the personal_time rows that intersect a
// single calendar date — used by availability to know what to subtract.
func (r *scheduleRepo) ListPersonalTimeOverlapping(ctx context.Context, userID uuid.UUID, date time.Time) ([]model.PersonalTime, error) {
	d := date.Format("2006-01-02")
	rows, err := r.pool.Query(ctx,
		`SELECT `+personalTimeColumns+`
		   FROM personal_time
		  WHERE user_id = $1 AND start_date <= $2::date AND end_date >= $2::date`,
		userID, d,
	)
	if err != nil {
		return nil, fmt.Errorf("listing overlapping personal_time: %w", err)
	}
	return r.collectPersonalTime(rows)
}

func (r *scheduleRepo) CreatePersonalTime(ctx context.Context, pt *model.PersonalTime) error {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO personal_time (user_id, start_date, end_date, start_time, end_time)
		 VALUES ($1, $2::date, $3::date, $4::time, $5::time)
		 RETURNING id, created_at, updated_at`,
		pt.UserID, pt.StartDate, pt.EndDate, pt.StartTime, pt.EndTime,
	)
	if err := row.Scan(&pt.ID, &pt.CreatedAt, &pt.UpdatedAt); err != nil {
		return fmt.Errorf("inserting personal_time: %w", err)
	}
	return nil
}

func (r *scheduleRepo) DeletePersonalTime(ctx context.Context, id uuid.UUID) error {
	cmd, err := r.pool.Exec(ctx, `DELETE FROM personal_time WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting personal_time: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
