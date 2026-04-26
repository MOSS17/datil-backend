package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mossandoval/datil-api/internal/middleware"
	"github.com/mossandoval/datil-api/internal/model"
	"github.com/mossandoval/datil-api/internal/repository"
	"github.com/mossandoval/datil-api/internal/storage"
)

const (
	appointmentListDefaultDays = 30
	appointmentListMaxDays     = 365
)

var allowedAppointmentStatuses = map[string]struct{}{
	"pending":   {},
	"confirmed": {},
	"cancelled": {},
	"completed": {},
}

type AppointmentHandler struct {
	repo         repository.AppointmentRepository
	businessRepo repository.BusinessRepository
	serviceRepo  repository.ServiceRepository
	uploader     storage.Uploader
	pool         *pgxpool.Pool
}

func NewAppointmentHandler(
	repo repository.AppointmentRepository,
	businessRepo repository.BusinessRepository,
	serviceRepo repository.ServiceRepository,
	uploader storage.Uploader,
	pool *pgxpool.Pool,
) *AppointmentHandler {
	return &AppointmentHandler{
		repo:         repo,
		businessRepo: businessRepo,
		serviceRepo:  serviceRepo,
		uploader:     uploader,
		pool:         pool,
	}
}

// loadOwned returns (appt, 0, nil) if the appointment exists and belongs to
// the authenticated user. On ErrNotFound → (nil, 404, nil). On cross-user
// access → (nil, 403, nil). Sufficient because each business has exactly
// one owner user in this MVP; revisit with multi-user-per-business.
func (h *AppointmentHandler) loadOwned(ctx context.Context, id uuid.UUID) (*model.Appointment, int, error) {
	appt, err := h.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, http.StatusNotFound, nil
		}
		return nil, 0, err
	}
	if appt.UserID != middleware.UserIDFromContext(ctx) {
		return nil, http.StatusForbidden, nil
	}
	return appt, 0, nil
}

func (h *AppointmentHandler) stitchServices(ctx context.Context, appts []model.Appointment) ([]model.Appointment, error) {
	if len(appts) == 0 {
		return appts, nil
	}
	ids := make([]uuid.UUID, len(appts))
	for i, a := range appts {
		ids[i] = a.ID
	}
	services, err := h.repo.ListServicesFor(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range appts {
		appts[i].Services = services[appts[i].ID]
	}
	return appts, nil
}

// List returns appointments filterable by date range.
// GET /appointments?from=YYYY-MM-DD&to=YYYY-MM-DD
func (h *AppointmentHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	businessID := middleware.BusinessIDFromContext(ctx)

	business, err := h.businessRepo.GetByID(ctx, businessID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not load business", nil)
		return
	}
	loc := model.BusinessLocation(business.Timezone)

	now := time.Now().In(loc)
	defaultFrom := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	defaultTo := defaultFrom.AddDate(0, 0, appointmentListDefaultDays)

	from, err := parseDateParam(r.URL.Query().Get("from"), loc, defaultFrom)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "datos inválidos", map[string]string{"from": "formato YYYY-MM-DD"})
		return
	}
	to, err := parseDateParam(r.URL.Query().Get("to"), loc, defaultTo)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "datos inválidos", map[string]string{"to": "formato YYYY-MM-DD"})
		return
	}
	if !to.After(from) {
		WriteError(w, http.StatusBadRequest, "datos inválidos", map[string]string{"to": "debe ser posterior a 'from'"})
		return
	}
	if to.Sub(from) > time.Duration(appointmentListMaxDays)*24*time.Hour {
		WriteError(w, http.StatusBadRequest, "datos inválidos", map[string]string{"to": "rango máximo 365 días"})
		return
	}

	appts, err := h.repo.List(ctx, userID, from, to)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not list appointments", nil)
		return
	}
	appts, err = h.stitchServices(ctx, appts)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not load appointment services", nil)
		return
	}
	WriteJSON(w, http.StatusOK, appts)
}

// Get returns a single appointment with its services.
// GET /appointments/{id}
func (h *AppointmentHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAppointmentID(w, r)
	if !ok {
		return
	}
	appt, status, err := h.loadOwned(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not load appointment", nil)
		return
	}
	if status != 0 {
		writeAppointmentStatus(w, status)
		return
	}
	services, err := h.repo.ListServicesFor(r.Context(), []uuid.UUID{appt.ID})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not load appointment services", nil)
		return
	}
	appt.Services = services[appt.ID]
	WriteJSON(w, http.StatusOK, appt)
}

// Create is an owner-initiated manual booking. No race guard — owners can
// knowingly double-book.
// POST /appointments
func (h *AppointmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req model.CreateAppointmentRequest
	if err := ReadJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "datos inválidos", nil)
		return
	}
	req.CustomerName = strings.TrimSpace(req.CustomerName)
	req.CustomerPhone = strings.TrimSpace(req.CustomerPhone)
	fields := map[string]string{}
	if req.CustomerName == "" {
		fields["customer_name"] = "requerido"
	}
	if req.CustomerPhone == "" {
		fields["customer_phone"] = "requerido"
	}
	if req.StartTime.IsZero() {
		fields["start_time"] = "requerido"
	}
	if !req.EndTime.IsZero() && !req.EndTime.After(req.StartTime) {
		fields["end_time"] = "debe ser posterior a start_time"
	}
	if len(req.ServiceIDs) == 0 {
		fields["service_ids"] = "requerido"
	}
	if len(fields) > 0 {
		WriteError(w, http.StatusBadRequest, "datos inválidos", fields)
		return
	}

	userID := middleware.UserIDFromContext(ctx)
	businessID := middleware.BusinessIDFromContext(ctx)

	apptServices, _, total, totalDuration, err := resolveBookableServices(ctx, h.serviceRepo, businessID, req.ServiceIDs, req.ExtraIDs)
	if err != nil {
		writeBookableServiceError(w, err)
		return
	}

	endTime := req.EndTime
	if endTime.IsZero() {
		endTime = req.StartTime.Add(time.Duration(totalDuration) * time.Minute)
	}

	appt := &model.Appointment{
		UserID:        userID,
		CustomerName:  req.CustomerName,
		CustomerEmail: req.CustomerEmail,
		CustomerPhone: req.CustomerPhone,
		StartTime:     req.StartTime,
		EndTime:       endTime,
		Total:         total,
		Status:        "confirmed",
	}

	err = repository.WithTransaction(ctx, h.pool, func(tx pgx.Tx) error {
		return h.repo.Create(ctx, tx, appt, apptServices)
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not create appointment", nil)
		return
	}

	WriteJSON(w, http.StatusCreated, appt)
}

// Update edits customer/time/total fields. Status is untouched (has its own
// endpoint); advance_payment_image_url is preserved.
// PUT /appointments/{id}
func (h *AppointmentHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAppointmentID(w, r)
	if !ok {
		return
	}
	var req model.UpdateAppointmentRequest
	if err := ReadJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "datos inválidos", nil)
		return
	}
	req.CustomerName = strings.TrimSpace(req.CustomerName)
	req.CustomerPhone = strings.TrimSpace(req.CustomerPhone)
	fields := map[string]string{}
	if req.CustomerName == "" {
		fields["customer_name"] = "requerido"
	}
	if req.CustomerPhone == "" {
		fields["customer_phone"] = "requerido"
	}
	if req.StartTime.IsZero() || req.EndTime.IsZero() || !req.EndTime.After(req.StartTime) {
		fields["end_time"] = "debe ser posterior a start_time"
	}
	if req.Total < 0 {
		fields["total"] = "no puede ser negativo"
	}
	if len(fields) > 0 {
		WriteError(w, http.StatusBadRequest, "datos inválidos", fields)
		return
	}

	appt, status, err := h.loadOwned(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not load appointment", nil)
		return
	}
	if status != 0 {
		writeAppointmentStatus(w, status)
		return
	}

	appt.CustomerName = req.CustomerName
	appt.CustomerEmail = req.CustomerEmail
	appt.CustomerPhone = req.CustomerPhone
	appt.StartTime = req.StartTime
	appt.EndTime = req.EndTime
	appt.Total = req.Total

	if err := h.repo.Update(r.Context(), id, appt); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "cita no encontrada", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "could not update appointment", nil)
		return
	}

	// Reload so we return canonical data (updated_at, etc.) with services.
	h.respondWithAppointment(w, r.Context(), id, http.StatusOK)
}

// UpdateStatus edits only the status field.
// PUT /appointments/{id}/status
func (h *AppointmentHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAppointmentID(w, r)
	if !ok {
		return
	}
	var req model.UpdateAppointmentStatusRequest
	if err := ReadJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "datos inválidos", nil)
		return
	}
	if _, valid := allowedAppointmentStatuses[req.Status]; !valid {
		WriteError(w, http.StatusBadRequest, "datos inválidos", map[string]string{"status": "valor no permitido"})
		return
	}

	_, status, err := h.loadOwned(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not load appointment", nil)
		return
	}
	if status != 0 {
		writeAppointmentStatus(w, status)
		return
	}

	if err := h.repo.UpdateStatus(r.Context(), id, req.Status); err != nil {
		WriteError(w, http.StatusInternalServerError, "could not update status", nil)
		return
	}
	h.respondWithAppointment(w, r.Context(), id, http.StatusOK)
}

// UpdatePaymentProof attaches (or replaces) the appointment's payment proof.
// POST /appointments/{id}/payment-proof  (multipart, field "payment_proof")
func (h *AppointmentHandler) UpdatePaymentProof(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAppointmentID(w, r)
	if !ok {
		return
	}

	// Ownership first so a rejected request doesn't waste upload bandwidth.
	_, status, err := h.loadOwned(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not load appointment", nil)
		return
	}
	if status != 0 {
		writeAppointmentStatus(w, status)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxReserveBytes+512)
	if err := r.ParseMultipartForm(maxReserveMemory); err != nil {
		WriteError(w, http.StatusBadRequest, "archivo demasiado grande o formato inválido", nil)
		return
	}
	file, hdr, err := r.FormFile("payment_proof")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "payment_proof requerido", nil)
		return
	}
	defer file.Close()

	ct, body, vErr := storage.DetectAndValidate(file, allowedPaymentProofTypes, maxReserveBytes)
	if vErr != nil {
		if errors.Is(vErr, storage.ErrInvalidContentType) {
			WriteError(w, http.StatusBadRequest, "formato no permitido (PNG, JPEG, WebP, PDF)", nil)
			return
		}
		WriteError(w, http.StatusBadRequest, "no se pudo leer el archivo", nil)
		return
	}

	businessID := middleware.BusinessIDFromContext(r.Context())
	key := fmt.Sprintf("appointments/%s/proof-%s", businessID, uuid.NewString())
	url, err := h.uploader.Upload(r.Context(), key, ct, hdr.Size, body)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "no se pudo subir el archivo", nil)
		return
	}

	if err := h.repo.UpdatePaymentProof(r.Context(), id, url); err != nil {
		WriteError(w, http.StatusInternalServerError, "could not update payment proof", nil)
		return
	}
	h.respondWithAppointment(w, r.Context(), id, http.StatusOK)
}

// Delete removes an appointment. FK ON DELETE CASCADE on appointment_services
// handles the join rows.
// DELETE /appointments/{id}
func (h *AppointmentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAppointmentID(w, r)
	if !ok {
		return
	}
	_, status, err := h.loadOwned(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not load appointment", nil)
		return
	}
	if status != 0 {
		writeAppointmentStatus(w, status)
		return
	}
	if err := h.repo.Delete(r.Context(), id); err != nil {
		WriteError(w, http.StatusInternalServerError, "could not delete appointment", nil)
		return
	}
	WriteNoContent(w)
}

// UnseenCount returns the count of recently-booked appointments the
// authenticated user has not yet opened. Drives the sidebar badge.
// GET /appointments/unseen-count
func (h *AppointmentHandler) UnseenCount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	count, err := h.repo.CountUnseenRecent(r.Context(), userID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not count unseen appointments", nil)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]int{"count": count})
}

// MarkSeen stamps seen_at so the frontend can stop showing the "new" pill
// for this appointment across devices. Idempotent: re-calling on an
// already-seen appointment preserves the original timestamp.
// POST /appointments/{id}/seen
func (h *AppointmentHandler) MarkSeen(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAppointmentID(w, r)
	if !ok {
		return
	}
	_, status, err := h.loadOwned(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not load appointment", nil)
		return
	}
	if status != 0 {
		writeAppointmentStatus(w, status)
		return
	}
	appt, err := h.repo.MarkSeen(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not mark appointment seen", nil)
		return
	}
	services, err := h.repo.ListServicesFor(r.Context(), []uuid.UUID{appt.ID})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not load appointment services", nil)
		return
	}
	appt.Services = services[appt.ID]
	WriteJSON(w, http.StatusOK, appt)
}

// --- helpers -----------------------------------------------------------------

func parseAppointmentID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := chi.URLParam(r, "id")
	id, err := uuid.Parse(raw)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "datos inválidos", map[string]string{"id": "uuid inválido"})
		return uuid.Nil, false
	}
	return id, true
}

func writeAppointmentStatus(w http.ResponseWriter, status int) {
	switch status {
	case http.StatusNotFound:
		WriteError(w, status, "cita no encontrada", nil)
	case http.StatusForbidden:
		WriteError(w, status, "no autorizado", nil)
	default:
		WriteError(w, http.StatusInternalServerError, "internal error", nil)
	}
}

// respondWithAppointment reloads the appointment + services and writes it.
// Use after a mutation so the response reflects canonical DB state.
func (h *AppointmentHandler) respondWithAppointment(w http.ResponseWriter, ctx context.Context, id uuid.UUID, status int) {
	appt, err := h.repo.GetByID(ctx, id)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not reload appointment", nil)
		return
	}
	services, err := h.repo.ListServicesFor(ctx, []uuid.UUID{id})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not load appointment services", nil)
		return
	}
	appt.Services = services[id]
	WriteJSON(w, status, appt)
}

// parseDateParam reads a YYYY-MM-DD string anchored to loc. Empty → fallback.
func parseDateParam(raw string, loc *time.Location, fallback time.Time) (time.Time, error) {
	if raw == "" {
		return fallback, nil
	}
	return time.ParseInLocation("2006-01-02", raw, loc)
}
