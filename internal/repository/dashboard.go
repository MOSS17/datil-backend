package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mossandoval/datil-api/internal/model"
)

type DashboardRepository interface {
	// GetDashboard returns counts for today/this-week, monthly income, and
	// the upcoming/latest appointment lists. Date boundaries are computed in
	// loc so the owner sees "today" anchored to their business's wall clock,
	// not the server's.
	//
	// businessID is accepted but not used in the SQL today because the
	// single-owner-per-business invariant means filtering on user_id is
	// equivalent. When the model grows to multiple users per business, swap
	// the appointments filter to `JOIN users u ON u.id = a.user_id WHERE
	// u.business_id = $N` and keep this signature stable.
	GetDashboard(ctx context.Context, userID, businessID uuid.UUID, loc *time.Location, upcomingLimit, latestLimit int) (*model.DashboardData, error)
}

type dashboardRepo struct {
	pool            *pgxpool.Pool
	appointmentRepo AppointmentRepository
}

func NewDashboardRepository(pool *pgxpool.Pool, appointmentRepo AppointmentRepository) DashboardRepository {
	return &dashboardRepo{pool: pool, appointmentRepo: appointmentRepo}
}

func (r *dashboardRepo) GetDashboard(ctx context.Context, userID, _ uuid.UUID, loc *time.Location, upcomingLimit, latestLimit int) (*model.DashboardData, error) {
	if loc == nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)

	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	todayEnd := todayStart.Add(24 * time.Hour)

	// Mon=0..Sun=6 for a Mon-Sun week.
	weekOffset := (int(now.Weekday()) + 6) % 7
	weekStart := todayStart.AddDate(0, 0, -weekOffset)
	weekEnd := weekStart.AddDate(0, 0, 7)

	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	monthEnd := monthStart.AddDate(0, 1, 0)

	data := &model.DashboardData{
		Upcoming: make([]model.Appointment, 0),
		Latest:   make([]model.Appointment, 0),
	}

	// Aggregate counts + monthly income. Outer WHERE spans the widest window
	// (min..max across the three ranges) so today/week FILTERs are subsets.
	// today and week both fall inside the current calendar month in the
	// common case; the LEAST/GREATEST keeps the bounds correct at month
	// edges (e.g. Sun Apr 30 → week continues into May).
	row := r.pool.QueryRow(ctx,
		`SELECT
		    COUNT(*) FILTER (WHERE start_time >= $2 AND start_time < $3) AS today_count,
		    COUNT(*) FILTER (WHERE start_time >= $4 AND start_time < $5) AS week_count,
		    COALESCE(SUM(total) FILTER (WHERE start_time >= $6 AND start_time < $7), 0) AS monthly_income
		   FROM appointments
		  WHERE user_id = $1
		    AND status <> 'cancelled'
		    AND start_time >= LEAST($2, $4, $6)
		    AND start_time <  GREATEST($3, $5, $7)`,
		userID,
		todayStart, todayEnd,
		weekStart, weekEnd,
		monthStart, monthEnd,
	)
	if err := row.Scan(&data.TodayCount, &data.WeekCount, &data.MonthlyIncome); err != nil {
		return nil, fmt.Errorf("dashboard counts: %w", err)
	}

	// Upcoming: next N future, non-cancelled.
	rows, err := r.pool.Query(ctx,
		`SELECT `+appointmentColumns+`
		   FROM appointments
		  WHERE user_id = $1
		    AND start_time >= $2
		    AND status <> 'cancelled'
		  ORDER BY start_time ASC
		  LIMIT $3`,
		userID, now, upcomingLimit,
	)
	if err != nil {
		return nil, fmt.Errorf("dashboard upcoming: %w", err)
	}
	upcoming, err := collectAppointments(rows)
	if err != nil {
		return nil, err
	}
	data.Upcoming = upcoming

	// Latest: unread notifications — recently-booked appointments the owner
	// hasn't opened yet. Bounded to the last 5 days of created_at so the
	// badge has a natural decay even if the owner never clicks. Clicking
	// the detail drawer calls POST /appointments/{id}/seen which flips
	// seen_at and filters the row out of subsequent reads.
	rows, err = r.pool.Query(ctx,
		`SELECT `+appointmentColumns+`
		   FROM appointments
		  WHERE user_id = $1
		    AND seen_at IS NULL
		    AND created_at >= ($2::timestamptz - INTERVAL '5 days')
		  ORDER BY created_at DESC
		  LIMIT $3`,
		userID, now, latestLimit,
	)
	if err != nil {
		return nil, fmt.Errorf("dashboard latest: %w", err)
	}
	latest, err := collectAppointments(rows)
	if err != nil {
		return nil, err
	}
	data.Latest = latest

	// Stitch services onto both lists in one round-trip.
	ids := make([]uuid.UUID, 0, len(data.Upcoming)+len(data.Latest))
	for _, a := range data.Upcoming {
		ids = append(ids, a.ID)
	}
	for _, a := range data.Latest {
		ids = append(ids, a.ID)
	}
	services, err := r.appointmentRepo.ListServicesFor(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range data.Upcoming {
		data.Upcoming[i].Services = services[data.Upcoming[i].ID]
	}
	for i := range data.Latest {
		data.Latest[i].Services = services[data.Latest[i].ID]
	}

	return data, nil
}
