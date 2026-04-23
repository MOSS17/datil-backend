package handler

import (
	"net/http"

	"github.com/mossandoval/datil-api/internal/notification"
	"github.com/mossandoval/datil-api/internal/repository"
)

type BookingHandler struct {
	businessRepo    repository.BusinessRepository
	serviceRepo     repository.ServiceRepository
	appointmentRepo repository.AppointmentRepository
	scheduleRepo    repository.ScheduleRepository
	notifier        notification.Notifier
}

func NewBookingHandler(
	businessRepo repository.BusinessRepository,
	serviceRepo repository.ServiceRepository,
	appointmentRepo repository.AppointmentRepository,
	scheduleRepo repository.ScheduleRepository,
	notifier notification.Notifier,
) *BookingHandler {
	return &BookingHandler{
		businessRepo:    businessRepo,
		serviceRepo:     serviceRepo,
		appointmentRepo: appointmentRepo,
		scheduleRepo:    scheduleRepo,
		notifier:        notifier,
	}
}

// GetBusiness returns the public page for a business.
// GET /book/{url}
func (h *BookingHandler) GetBusiness(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}

// GetServices returns available services grouped by category.
// GET /book/{url}/services
func (h *BookingHandler) GetServices(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}

// GetAvailability returns available time slots for a date.
// GET /book/{url}/availability?date=YYYY-MM-DD
func (h *BookingHandler) GetAvailability(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}

// Reserve creates a new reservation from the public booking page.
// POST /book/{url}/reserve
func (h *BookingHandler) Reserve(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}
