package handler

import (
	"net/http"

	"github.com/mossandoval/datil-api/internal/repository"
)

type BusinessHandler struct {
	repo repository.BusinessRepository
}

func NewBusinessHandler(repo repository.BusinessRepository) *BusinessHandler {
	return &BusinessHandler{repo: repo}
}

// Get returns the current user's business.
// GET /business
func (h *BusinessHandler) Get(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}

// Update updates the current user's business details.
// PUT /business
func (h *BusinessHandler) Update(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}

// UpdateBank updates the bank details for the current user's business.
// PUT /business/bank
func (h *BusinessHandler) UpdateBank(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}

// UpdateLogo uploads a logo for the current user's business.
// PUT /business/logo
func (h *BusinessHandler) UpdateLogo(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}
