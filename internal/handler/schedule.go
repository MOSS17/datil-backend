package handler

import (
	"net/http"

	"github.com/mossandoval/datil-api/internal/repository"
)

type ScheduleHandler struct {
	repo repository.ScheduleRepository
}

func NewScheduleHandler(repo repository.ScheduleRepository) *ScheduleHandler {
	return &ScheduleHandler{repo: repo}
}

// GetWorkdays returns all 7 workdays with their hours.
// GET /schedule/workdays
func (h *ScheduleHandler) GetWorkdays(w http.ResponseWriter, r *http.Request) {
	ErrorJSON(w, http.StatusNotImplemented, "not implemented")
}

// UpdateWorkdays bulk updates workdays (enabled/disabled + hours).
// PUT /schedule/workdays
func (h *ScheduleHandler) UpdateWorkdays(w http.ResponseWriter, r *http.Request) {
	ErrorJSON(w, http.StatusNotImplemented, "not implemented")
}

// ListPersonalTime returns all personal time blocks for the current user.
// GET /schedule/personal-time
func (h *ScheduleHandler) ListPersonalTime(w http.ResponseWriter, r *http.Request) {
	ErrorJSON(w, http.StatusNotImplemented, "not implemented")
}

// CreatePersonalTime creates a new personal time block.
// POST /schedule/personal-time
func (h *ScheduleHandler) CreatePersonalTime(w http.ResponseWriter, r *http.Request) {
	ErrorJSON(w, http.StatusNotImplemented, "not implemented")
}

// DeletePersonalTime deletes a personal time block.
// DELETE /schedule/personal-time/{id}
func (h *ScheduleHandler) DeletePersonalTime(w http.ResponseWriter, r *http.Request) {
	ErrorJSON(w, http.StatusNotImplemented, "not implemented")
}
