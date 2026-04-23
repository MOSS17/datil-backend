package handler

import (
	"net/http"

	"github.com/mossandoval/datil-api/internal/notification"
	"github.com/mossandoval/datil-api/internal/repository"
)

type AppointmentHandler struct {
	repo     repository.AppointmentRepository
	notifier notification.Notifier
}

func NewAppointmentHandler(repo repository.AppointmentRepository, notifier notification.Notifier) *AppointmentHandler {
	return &AppointmentHandler{repo: repo, notifier: notifier}
}

// List returns appointments filterable by date range.
// GET /appointments
func (h *AppointmentHandler) List(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}

// Get returns a single appointment with its services.
// GET /appointments/{id}
func (h *AppointmentHandler) Get(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}

// Create creates a new appointment (owner-created).
// POST /appointments
func (h *AppointmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}

// Update updates an appointment.
// PUT /appointments/{id}
func (h *AppointmentHandler) Update(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}

// Delete cancels/deletes an appointment.
// DELETE /appointments/{id}
func (h *AppointmentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}
