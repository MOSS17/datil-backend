package handler

import (
	"net/http"

	"github.com/mossandoval/datil-api/internal/repository"
)

type CalendarHandler struct {
	repo repository.CalendarRepository
}

func NewCalendarHandler(repo repository.CalendarRepository) *CalendarHandler {
	return &CalendarHandler{repo: repo}
}

// Connect initiates the OAuth flow for a calendar provider.
// POST /calendar/{provider}/connect
func (h *CalendarHandler) Connect(w http.ResponseWriter, r *http.Request) {
	ErrorJSON(w, http.StatusNotImplemented, "not implemented")
}

// Callback handles the OAuth callback from a calendar provider.
// GET /calendar/{provider}/callback
func (h *CalendarHandler) Callback(w http.ResponseWriter, r *http.Request) {
	ErrorJSON(w, http.StatusNotImplemented, "not implemented")
}

// Disconnect removes a calendar integration.
// DELETE /calendar/{provider}
func (h *CalendarHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	ErrorJSON(w, http.StatusNotImplemented, "not implemented")
}
