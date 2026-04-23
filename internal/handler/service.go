package handler

import (
	"net/http"

	"github.com/mossandoval/datil-api/internal/repository"
)

type ServiceHandler struct {
	repo repository.ServiceRepository
}

func NewServiceHandler(repo repository.ServiceRepository) *ServiceHandler {
	return &ServiceHandler{repo: repo}
}

// List returns all services for the current user's business.
// GET /services
func (h *ServiceHandler) List(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}

// Get returns a service with its available extras.
// GET /services/{id}
func (h *ServiceHandler) Get(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}

// Create creates a new service.
// POST /services
func (h *ServiceHandler) Create(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}

// Update updates a service.
// PUT /services/{id}
func (h *ServiceHandler) Update(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}

// Delete deletes a service.
// DELETE /services/{id}
func (h *ServiceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}

// LinkExtra links an extra service to a main service.
// POST /services/{id}/extras
func (h *ServiceHandler) LinkExtra(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}

// UnlinkExtra unlinks an extra service from a main service.
// DELETE /services/{id}/extras/{extraId}
func (h *ServiceHandler) UnlinkExtra(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}
