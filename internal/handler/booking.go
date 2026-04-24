package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mossandoval/datil-api/internal/booking"
	"github.com/mossandoval/datil-api/internal/calendar"
	"github.com/mossandoval/datil-api/internal/model"
	"github.com/mossandoval/datil-api/internal/notification"
	"github.com/mossandoval/datil-api/internal/repository"
	"github.com/mossandoval/datil-api/internal/storage"
)

type BookingHandler struct {
	businessRepo    repository.BusinessRepository
	userRepo        repository.UserRepository
	categoryRepo    repository.CategoryRepository
	serviceRepo     repository.ServiceRepository
	appointmentRepo repository.AppointmentRepository
	scheduleRepo    repository.ScheduleRepository
	calendarRepo    repository.CalendarRepository
	uploader        storage.Uploader
	notifier        notification.Notifier
	calSyncer       calendar.Syncer
	pool            *pgxpool.Pool
}

func NewBookingHandler(
	businessRepo repository.BusinessRepository,
	userRepo repository.UserRepository,
	categoryRepo repository.CategoryRepository,
	serviceRepo repository.ServiceRepository,
	appointmentRepo repository.AppointmentRepository,
	scheduleRepo repository.ScheduleRepository,
	calendarRepo repository.CalendarRepository,
	uploader storage.Uploader,
	notifier notification.Notifier,
	calSyncer calendar.Syncer,
	pool *pgxpool.Pool,
) *BookingHandler {
	return &BookingHandler{
		businessRepo:    businessRepo,
		userRepo:        userRepo,
		categoryRepo:    categoryRepo,
		serviceRepo:     serviceRepo,
		appointmentRepo: appointmentRepo,
		scheduleRepo:    scheduleRepo,
		calendarRepo:    calendarRepo,
		uploader:        uploader,
		notifier:        notifier,
		calSyncer:       calSyncer,
		pool:            pool,
	}
}

const availabilitySlotStep = 15

// GetBusiness returns the public-facing business + its categories by slug.
// GET /book/{url}
func (h *BookingHandler) GetBusiness(w http.ResponseWriter, r *http.Request) {
	url := chi.URLParam(r, "url")
	business, err := h.businessRepo.GetByURL(r.Context(), url)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "negocio no encontrado", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "could not load business", nil)
		return
	}
	cats, err := h.categoryRepo.List(r.Context(), business.ID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not load categories", nil)
		return
	}
	WriteJSON(w, http.StatusOK, model.BookingPageResponse{
		Business:   *business,
		Categories: cats,
	})
}

// GetServices returns active services + their linked extras for the business.
// Frontend groups by category_id and is_extra; backend keeps the response flat.
// GET /book/{url}/services
func (h *BookingHandler) GetServices(w http.ResponseWriter, r *http.Request) {
	url := chi.URLParam(r, "url")
	services, err := h.serviceRepo.ListByBusinessURL(r.Context(), url)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not load services", nil)
		return
	}

	// Per-service extras lookup. N+1 by design; the booking page is a one-off
	// load, not a hot path. If it ever matters, replace with a single JOIN.
	out := make([]model.BookingService, 0, len(services))
	for _, s := range services {
		extras, err := h.serviceRepo.ListExtras(r.Context(), s.ID)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "could not load extras", nil)
			return
		}
		out = append(out, model.BookingService{Service: s, Extras: extras})
	}
	WriteJSON(w, http.StatusOK, out)
}

// GetAvailability computes bookable start times for a date + service bundle.
// GET /book/{url}/availability?date=YYYY-MM-DD&service_ids=uuid,uuid,...
func (h *BookingHandler) GetAvailability(w http.ResponseWriter, r *http.Request) {
	url := chi.URLParam(r, "url")
	business, err := h.businessRepo.GetByURL(r.Context(), url)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "negocio no encontrado", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "could not load business", nil)
		return
	}

	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		WriteError(w, http.StatusBadRequest, "date requerido (YYYY-MM-DD)", nil)
		return
	}
	loc := model.BusinessLocation(business.Timezone)
	date, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "date inválido (YYYY-MM-DD)", nil)
		return
	}

	serviceIDs, err := parseUUIDList(r.URL.Query().Get("service_ids"))
	if err != nil || len(serviceIDs) == 0 {
		WriteError(w, http.StatusBadRequest, "service_ids requerido", nil)
		return
	}

	totalDuration := 0
	for _, id := range serviceIDs {
		s, err := h.serviceRepo.GetByID(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "service_id inválido", nil)
			return
		}
		if s.BusinessID != business.ID {
			WriteError(w, http.StatusBadRequest, "service_id no pertenece a este negocio", nil)
			return
		}
		totalDuration += s.Duration
	}

	owner, err := h.userRepo.GetByBusinessID(r.Context(), business.ID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not load business owner", nil)
		return
	}

	workdays, err := h.scheduleRepo.ListWorkdays(r.Context(), business.ID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not load workdays", nil)
		return
	}
	workday := pickWorkday(workdays, int(date.Weekday()))

	personal, err := h.scheduleRepo.ListPersonalTimeOverlapping(r.Context(), owner.ID, date)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not load personal time", nil)
		return
	}

	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dayEnd := dayStart.Add(24 * time.Hour)
	appts, err := h.appointmentRepo.ListByDateRange(r.Context(), business.ID, dayStart, dayEnd)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not load appointments", nil)
		return
	}

	slots := booking.ComputeSlots(workday, personal, appts, totalDuration, dayStart, availabilitySlotStep)
	WriteJSON(w, http.StatusOK, slots)
}

// Reserve creates an appointment from the public booking page. Multipart
// body with optional payment_proof file. Inside a Postgres tx, locks the
// day's appointments FOR UPDATE to prevent concurrent reservations from
// double-booking the same start time.
// POST /book/{url}/reserve
func (h *BookingHandler) Reserve(w http.ResponseWriter, r *http.Request) {
	url := chi.URLParam(r, "url")
	business, err := h.businessRepo.GetByURL(r.Context(), url)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "negocio no encontrado", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "could not load business", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxReserveBytes+512)
	if err := r.ParseMultipartForm(maxReserveMemory); err != nil {
		WriteError(w, http.StatusBadRequest, "no se pudo leer el formulario (máx 6 MB)", nil)
		return
	}

	customerName := strings.TrimSpace(r.FormValue("customer_name"))
	customerPhone := strings.TrimSpace(r.FormValue("customer_phone"))
	customerEmail := strings.TrimSpace(r.FormValue("customer_email"))
	startTimeStr := strings.TrimSpace(r.FormValue("start_time"))

	fields := map[string]string{}
	if customerName == "" {
		fields["customer_name"] = "requerido"
	}
	if customerPhone == "" {
		fields["customer_phone"] = "requerido"
	}
	if startTimeStr == "" {
		fields["start_time"] = "requerido"
	}
	if len(fields) > 0 {
		WriteError(w, http.StatusBadRequest, "datos inválidos", fields)
		return
	}

	// customer_email is optional. Frontend may omit it for walk-in style
	// bookings; persist as NULL when blank.
	var customerEmailPtr *string
	if customerEmail != "" {
		customerEmailPtr = &customerEmail
	}

	startTime, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "datos inválidos", map[string]string{"start_time": "formato RFC3339"})
		return
	}

	serviceIDs, err := parseUUIDList(strings.Join(r.Form["service_ids"], ","))
	if err != nil || len(serviceIDs) == 0 {
		WriteError(w, http.StatusBadRequest, "datos inválidos", map[string]string{"service_ids": "requerido"})
		return
	}
	extraIDs, _ := parseUUIDList(strings.Join(r.Form["extra_ids"], ","))

	// Resolve services up front so we can validate ownership, sum the price/
	// duration, build the appointment_services rows, and keep the names
	// around for the WhatsApp message body.
	apptServices, serviceNames, total, totalDuration, err := resolveBookableServices(r.Context(), h.serviceRepo, business.ID, serviceIDs, extraIDs)
	if err != nil {
		writeBookableServiceError(w, err)
		return
	}

	endTime := startTime.Add(time.Duration(totalDuration) * time.Minute)

	owner, err := h.userRepo.GetByBusinessID(r.Context(), business.ID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not load business owner", nil)
		return
	}

	// Optional payment proof. Sniff content type before uploading.
	var paymentProofURL *string
	if file, hdr, err := r.FormFile("payment_proof"); err == nil {
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
		key := fmt.Sprintf("appointments/%s/proof-%s", business.ID, uuid.NewString())
		uploadedURL, uErr := h.uploader.Upload(r.Context(), key, ct, hdr.Size, body)
		if uErr != nil {
			WriteError(w, http.StatusInternalServerError, "no se pudo subir el archivo", nil)
			return
		}
		paymentProofURL = &uploadedURL
	}

	appt := &model.Appointment{
		UserID:                 owner.ID,
		CustomerName:           customerName,
		CustomerEmail:          customerEmailPtr,
		StartTime:              startTime,
		EndTime:                endTime,
		Total:                  total,
		CustomerPhone:          customerPhone,
		AdvancePaymentImageURL: paymentProofURL,
	}

	dayStart := time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, startTime.Location())
	dayEnd := dayStart.Add(24 * time.Hour)

	err = repository.WithTransaction(r.Context(), h.pool, func(tx pgx.Tx) error {
		// Lock the day's appointments and re-check for overlap. This is the
		// race guard: two concurrent reserves on the same slot will serialize
		// here, and the second one will see the first row and bail out.
		existing, err := h.appointmentRepo.ListByDateRangeForUpdate(r.Context(), tx, business.ID, dayStart, dayEnd)
		if err != nil {
			return err
		}
		for _, e := range existing {
			if startTime.Before(e.EndTime) && endTime.After(e.StartTime) {
				return errSlotTaken
			}
		}
		return h.appointmentRepo.Create(r.Context(), tx, appt, apptServices)
	})
	if err != nil {
		if errors.Is(err, errSlotTaken) {
			WriteError(w, http.StatusConflict, "ese horario ya no está disponible", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "could not create reservation", nil)
		return
	}

	// Post-commit: notify out-of-band so a flaky Twilio call doesn't fail
	// the response or rollback the appointment. Background context — request
	// context is already finishing.
	go func(phone, businessName, customerName string, start time.Time, services []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := h.notifier.SendBookingConfirmation(ctx, phone, model.BookingDetails{
			CustomerName: customerName,
			BusinessName: businessName,
			StartTime:    start,
			Services:     services,
		}); err != nil {
			log.Printf("notify booking: %v", err)
		}
	}(customerPhone, business.Name, customerName, startTime, serviceNames)

	// Calendar push: mirror the notifier pattern — best-effort, fire-and-
	// forget so a provider outage can't rollback the booking. One outer
	// goroutine lists the owner's active integrations; each integration
	// pushes in its own goroutine so Apple + Google run concurrently.
	if h.calSyncer != nil {
		go h.pushAppointmentToCalendars(*appt, business, customerName, customerPhone, customerEmail, serviceNames)
	}

	WriteJSON(w, http.StatusOK, appt)
}

func (h *BookingHandler) pushAppointmentToCalendars(appt model.Appointment, business *model.Business, customerName, customerPhone, customerEmail string, serviceNames []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	integrations, err := h.calendarRepo.ListByUser(ctx, appt.UserID)
	if err != nil {
		log.Printf("list calendar integrations: %v", err)
		return
	}
	if len(integrations) == 0 {
		return
	}

	input := calendar.EventInput{
		BusinessName:  business.Name,
		CustomerName:  customerName,
		CustomerPhone: customerPhone,
		CustomerEmail: customerEmail,
		Services:      serviceNames,
		Start:         appt.StartTime,
		End:           appt.EndTime,
		Timezone:      business.Timezone,
	}
	for _, ci := range integrations {
		externalID, err := h.calSyncer.PushEvent(ctx, ci, input)
		if err != nil {
			log.Printf("calendar push (%s): %v", ci.Provider, err)
			continue
		}
		if externalID == "" {
			continue
		}
		if err := h.appointmentRepo.UpdateExternalEventID(ctx, appt.ID, ci.Provider, externalID); err != nil {
			log.Printf("stamp %s event id: %v", ci.Provider, err)
		}
	}
}

var errSlotTaken = errors.New("slot taken")

func parseUUIDList(s string) ([]uuid.UUID, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]uuid.UUID, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := uuid.Parse(p)
		if err != nil {
			return nil, fmt.Errorf("invalid uuid %q: %w", p, err)
		}
		out = append(out, id)
	}
	return out, nil
}

// pickWorkday returns the Workday matching weekday (0=Sunday..6=Saturday) or
// a disabled placeholder if the business hasn't configured that day.
func pickWorkday(workdays []model.Workday, weekday int) model.Workday {
	for _, d := range workdays {
		if d.Day == weekday {
			return d
		}
	}
	return model.Workday{Day: weekday, IsEnabled: false}
}

