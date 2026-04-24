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

type AppointmentRepository interface {
	List(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.Appointment, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Appointment, error)
	Create(ctx context.Context, tx pgx.Tx, a *model.Appointment, services []model.AppointmentService) error
	Update(ctx context.Context, id uuid.UUID, a *model.Appointment) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
	UpdatePaymentProof(ctx context.Context, id uuid.UUID, url string) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByDateRange(ctx context.Context, businessID uuid.UUID, from, to time.Time) ([]model.Appointment, error)
	// ListByDateRangeForUpdate is like ListByDateRange but locks the matching
	// rows FOR UPDATE inside the calling transaction. Use it as the race
	// guard inside Reserve so two concurrent reservations can't both succeed.
	ListByDateRangeForUpdate(ctx context.Context, tx pgx.Tx, businessID uuid.UUID, from, to time.Time) ([]model.Appointment, error)
	// ListServicesFor returns the appointment_services rows for the given
	// appointment IDs, keyed by appointment_id. Empty input returns an empty
	// map without a query round-trip.
	ListServicesFor(ctx context.Context, appointmentIDs []uuid.UUID) (map[uuid.UUID][]model.AppointmentService, error)
}

type appointmentRepo struct {
	pool *pgxpool.Pool
}

func NewAppointmentRepository(pool *pgxpool.Pool) AppointmentRepository {
	return &appointmentRepo{pool: pool}
}

const appointmentColumns = "id, user_id, customer_name, customer_email, start_time, end_time, total, customer_phone, advance_payment_image_url, status, created_at, updated_at"

func scanAppointment(row pgx.Row) (*model.Appointment, error) {
	var a model.Appointment
	if err := row.Scan(
		&a.ID, &a.UserID, &a.CustomerName, &a.CustomerEmail, &a.StartTime, &a.EndTime,
		&a.Total, &a.CustomerPhone, &a.AdvancePaymentImageURL, &a.Status,
		&a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scanning appointment: %w", err)
	}
	return &a, nil
}

func collectAppointments(rows pgx.Rows) ([]model.Appointment, error) {
	defer rows.Close()
	out := make([]model.Appointment, 0)
	for rows.Next() {
		a, err := scanAppointment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating appointments: %w", err)
	}
	return out, nil
}

func (r *appointmentRepo) List(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.Appointment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+appointmentColumns+`
		   FROM appointments
		  WHERE user_id = $1 AND start_time >= $2 AND start_time < $3
		  ORDER BY start_time`,
		userID, from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("listing appointments: %w", err)
	}
	return collectAppointments(rows)
}

func (r *appointmentRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Appointment, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+appointmentColumns+` FROM appointments WHERE id = $1`, id)
	return scanAppointment(row)
}

// Create inserts the appointment and its services atomically. Caller must
// supply an open transaction so the booking-flow race guard
// (ListByDateRangeForUpdate) and the insert share a single Postgres tx.
// If a.Status is empty the DB default ('confirmed') applies.
func (r *appointmentRepo) Create(ctx context.Context, tx pgx.Tx, a *model.Appointment, services []model.AppointmentService) error {
	status := a.Status
	if status == "" {
		status = "confirmed"
	}
	row := tx.QueryRow(ctx,
		`INSERT INTO appointments
		    (user_id, customer_name, customer_email, start_time, end_time, total, customer_phone, advance_payment_image_url, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, status, created_at, updated_at`,
		a.UserID, a.CustomerName, a.CustomerEmail, a.StartTime, a.EndTime, a.Total, a.CustomerPhone, a.AdvancePaymentImageURL, status,
	)
	if err := row.Scan(&a.ID, &a.Status, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return fmt.Errorf("inserting appointment: %w", err)
	}

	for i := range services {
		services[i].AppointmentID = a.ID
		_, err := tx.Exec(ctx,
			`INSERT INTO appointment_services (appointment_id, service_id, price, duration)
			 VALUES ($1, $2, $3, $4)`,
			a.ID, services[i].ServiceID, services[i].Price, services[i].Duration,
		)
		if err != nil {
			return fmt.Errorf("inserting appointment_service: %w", err)
		}
	}
	a.Services = services
	return nil
}

func (r *appointmentRepo) Update(ctx context.Context, id uuid.UUID, a *model.Appointment) error {
	cmd, err := r.pool.Exec(ctx,
		`UPDATE appointments
		    SET customer_name = $1, customer_email = $2, start_time = $3, end_time = $4, total = $5,
		        customer_phone = $6, advance_payment_image_url = $7, updated_at = NOW()
		  WHERE id = $8`,
		a.CustomerName, a.CustomerEmail, a.StartTime, a.EndTime, a.Total, a.CustomerPhone, a.AdvancePaymentImageURL, id,
	)
	if err != nil {
		return fmt.Errorf("updating appointment: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *appointmentRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	cmd, err := r.pool.Exec(ctx,
		`UPDATE appointments SET status = $1, updated_at = NOW() WHERE id = $2`,
		status, id,
	)
	if err != nil {
		return fmt.Errorf("updating appointment status: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *appointmentRepo) UpdatePaymentProof(ctx context.Context, id uuid.UUID, url string) error {
	cmd, err := r.pool.Exec(ctx,
		`UPDATE appointments SET advance_payment_image_url = $1, updated_at = NOW() WHERE id = $2`,
		url, id,
	)
	if err != nil {
		return fmt.Errorf("updating appointment payment proof: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *appointmentRepo) Delete(ctx context.Context, id uuid.UUID) error {
	cmd, err := r.pool.Exec(ctx, `DELETE FROM appointments WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting appointment: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *appointmentRepo) ListServicesFor(ctx context.Context, appointmentIDs []uuid.UUID) (map[uuid.UUID][]model.AppointmentService, error) {
	out := make(map[uuid.UUID][]model.AppointmentService, len(appointmentIDs))
	if len(appointmentIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT appointment_id, service_id, price, duration
		   FROM appointment_services
		  WHERE appointment_id = ANY($1)`,
		appointmentIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("listing appointment services: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var s model.AppointmentService
		if err := rows.Scan(&s.AppointmentID, &s.ServiceID, &s.Price, &s.Duration); err != nil {
			return nil, fmt.Errorf("scanning appointment service: %w", err)
		}
		out[s.AppointmentID] = append(out[s.AppointmentID], s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating appointment services: %w", err)
	}
	return out, nil
}

// qualifiedAppointmentColumns mirrors appointmentColumns but prefixes every
// field with the `a.` alias. Required whenever the SELECT joins another
// table with overlapping names (users shares id/created_at/updated_at).
const qualifiedAppointmentColumns = "a.id, a.user_id, a.customer_name, a.customer_email, a.start_time, a.end_time, a.total, a.customer_phone, a.advance_payment_image_url, a.status, a.created_at, a.updated_at"

const appointmentByBusinessSQL = `SELECT ` + qualifiedAppointmentColumns + `
		   FROM appointments a
		   JOIN users u ON u.id = a.user_id
		  WHERE u.business_id = $1
		    AND a.start_time < $3 AND a.end_time > $2
		  ORDER BY a.start_time`

func (r *appointmentRepo) ListByDateRange(ctx context.Context, businessID uuid.UUID, from, to time.Time) ([]model.Appointment, error) {
	rows, err := r.pool.Query(ctx, appointmentByBusinessSQL, businessID, from, to)
	if err != nil {
		return nil, fmt.Errorf("listing appointments by range: %w", err)
	}
	return collectAppointments(rows)
}

func (r *appointmentRepo) ListByDateRangeForUpdate(ctx context.Context, tx pgx.Tx, businessID uuid.UUID, from, to time.Time) ([]model.Appointment, error) {
	rows, err := tx.Query(ctx, appointmentByBusinessSQL+" FOR UPDATE OF a", businessID, from, to)
	if err != nil {
		return nil, fmt.Errorf("locking appointments by range: %w", err)
	}
	return collectAppointments(rows)
}
