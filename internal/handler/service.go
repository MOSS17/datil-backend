package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mossandoval/datil-api/internal/middleware"
	"github.com/mossandoval/datil-api/internal/model"
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
	businessID := middleware.BusinessIDFromContext(r.Context())
	services, err := h.repo.List(r.Context(), businessID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not list services", nil)
		return
	}
	WriteJSON(w, http.StatusOK, services)
}

// Get returns a single service the caller owns.
// GET /services/{id}
func (h *ServiceHandler) Get(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.loadOwned(w, r, "id")
	if !ok {
		return
	}
	WriteJSON(w, http.StatusOK, svc)
}

// Create creates a new service for the caller's business.
// POST /services
func (h *ServiceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.ServiceRequest
	if err := ReadJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	if fields := validateService(req); len(fields) > 0 {
		WriteError(w, http.StatusBadRequest, "datos inválidos", fields)
		return
	}

	businessID := middleware.BusinessIDFromContext(r.Context())
	svc := &model.Service{
		BusinessID:           businessID,
		CategoryID:           req.CategoryID,
		Name:                 strings.TrimSpace(req.Name),
		Description:          req.Description,
		MinPrice:             req.MinPrice,
		MaxPrice:             req.MaxPrice,
		Duration:             req.Duration,
		AdvancePaymentAmount: req.AdvancePaymentAmount,
		IsExtra:              req.IsExtra,
		IsActive:             true,
	}
	if req.IsActive != nil {
		svc.IsActive = *req.IsActive
	}
	if err := h.repo.Create(r.Context(), svc); err != nil {
		WriteError(w, http.StatusInternalServerError, "could not create service", nil)
		return
	}
	WriteJSON(w, http.StatusOK, svc)
}

// Update edits a service the caller owns.
// PUT /services/{id}
func (h *ServiceHandler) Update(w http.ResponseWriter, r *http.Request) {
	current, ok := h.loadOwned(w, r, "id")
	if !ok {
		return
	}

	var req model.ServiceRequest
	if err := ReadJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	if fields := validateService(req); len(fields) > 0 {
		WriteError(w, http.StatusBadRequest, "datos inválidos", fields)
		return
	}

	current.CategoryID = req.CategoryID
	current.Name = strings.TrimSpace(req.Name)
	current.Description = req.Description
	current.MinPrice = req.MinPrice
	current.MaxPrice = req.MaxPrice
	current.Duration = req.Duration
	current.AdvancePaymentAmount = req.AdvancePaymentAmount
	current.IsExtra = req.IsExtra
	if req.IsActive != nil {
		current.IsActive = *req.IsActive
	}
	if err := h.repo.Update(r.Context(), current.ID, current); err != nil {
		WriteError(w, http.StatusInternalServerError, "could not update service", nil)
		return
	}

	updated, err := h.repo.GetByID(r.Context(), current.ID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not reload service", nil)
		return
	}
	WriteJSON(w, http.StatusOK, updated)
}

// Delete removes a service the caller owns.
// DELETE /services/{id}
func (h *ServiceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.loadOwned(w, r, "id")
	if !ok {
		return
	}
	if err := h.repo.Delete(r.Context(), svc.ID); err != nil {
		WriteError(w, http.StatusInternalServerError, "could not delete service", nil)
		return
	}
	WriteNoContent(w)
}

// ListExtras returns the extras linked to a service the caller owns.
// GET /services/{id}/extras
func (h *ServiceHandler) ListExtras(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.loadOwned(w, r, "id")
	if !ok {
		return
	}
	extras, err := h.repo.ListExtras(r.Context(), svc.ID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not list extras", nil)
		return
	}
	WriteJSON(w, http.StatusOK, extras)
}

// LinkExtra attaches an extra to a service. Both must belong to the caller's
// business and the extra service must have is_extra=true.
// POST /services/{id}/extras
func (h *ServiceHandler) LinkExtra(w http.ResponseWriter, r *http.Request) {
	parent, ok := h.loadOwned(w, r, "id")
	if !ok {
		return
	}

	var req model.LinkExtraRequest
	if err := ReadJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	if req.ExtraID == uuid.Nil {
		WriteError(w, http.StatusBadRequest, "datos inválidos", map[string]string{"extra_id": "requerido"})
		return
	}
	if req.ExtraID == parent.ID {
		WriteError(w, http.StatusBadRequest, "no se puede vincular un servicio a sí mismo", nil)
		return
	}

	extra, err := h.loadByIDOwned(r.Context(), req.ExtraID, middleware.BusinessIDFromContext(r.Context()))
	if err != nil {
		writeOwnershipError(w, err)
		return
	}
	if !extra.IsExtra {
		WriteError(w, http.StatusBadRequest, "el servicio destino no es un extra", nil)
		return
	}

	if err := h.repo.LinkExtra(r.Context(), parent.ID, extra.ID); err != nil {
		WriteError(w, http.StatusInternalServerError, "could not link extra", nil)
		return
	}
	WriteNoContent(w)
}

// UnlinkExtra detaches an extra from a service.
// DELETE /services/{id}/extras/{extraId}
func (h *ServiceHandler) UnlinkExtra(w http.ResponseWriter, r *http.Request) {
	parent, ok := h.loadOwned(w, r, "id")
	if !ok {
		return
	}
	extraID, err := uuid.Parse(chi.URLParam(r, "extraId"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "id inválido", nil)
		return
	}
	if err := h.repo.UnlinkExtra(r.Context(), parent.ID, extraID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "vínculo no encontrado", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "could not unlink extra", nil)
		return
	}
	WriteNoContent(w)
}

// loadOwned reads the {param} URL param, fetches the service, and verifies
// it belongs to the caller's business. Writes the appropriate error response
// and returns ok=false on failure.
func (h *ServiceHandler) loadOwned(w http.ResponseWriter, r *http.Request, param string) (*model.Service, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "id inválido", nil)
		return nil, false
	}
	businessID := middleware.BusinessIDFromContext(r.Context())
	svc, err := h.loadByIDOwned(r.Context(), id, businessID)
	if err != nil {
		writeOwnershipError(w, err)
		return nil, false
	}
	return svc, true
}

func (h *ServiceHandler) loadByIDOwned(ctx context.Context, id, businessID uuid.UUID) (*model.Service, error) {
	svc, err := h.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if svc.BusinessID != businessID {
		return nil, errForbidden
	}
	return svc, nil
}

var errForbidden = errors.New("forbidden")

func writeOwnershipError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		WriteError(w, http.StatusNotFound, "servicio no encontrado", nil)
	case errors.Is(err, errForbidden):
		WriteError(w, http.StatusForbidden, "no autorizado", nil)
	default:
		WriteError(w, http.StatusInternalServerError, "could not load service", nil)
	}
}

func validateService(req model.ServiceRequest) map[string]string {
	fields := map[string]string{}
	if strings.TrimSpace(req.Name) == "" {
		fields["name"] = "requerido"
	}
	if req.CategoryID == uuid.Nil {
		fields["category_id"] = "requerido"
	}
	if req.MinPrice < 0 {
		fields["min_price"] = "no puede ser negativo"
	}
	if req.MaxPrice != nil && *req.MaxPrice < req.MinPrice {
		fields["max_price"] = "debe ser >= min_price"
	}
	if req.Duration <= 0 {
		fields["duration"] = "debe ser > 0"
	}
	if req.AdvancePaymentAmount != nil && *req.AdvancePaymentAmount < 0 {
		fields["advance_payment_amount"] = "no puede ser negativo"
	}
	return fields
}
