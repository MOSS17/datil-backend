package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mossandoval/datil-api/internal/middleware"
	"github.com/mossandoval/datil-api/internal/model"
	"github.com/mossandoval/datil-api/internal/repository"
)

type CategoryHandler struct {
	repo repository.CategoryRepository
}

func NewCategoryHandler(repo repository.CategoryRepository) *CategoryHandler {
	return &CategoryHandler{repo: repo}
}

const categoryNameTakenMessage = "Ya existe una categoría con ese nombre"

// List returns every category owned by the caller's business.
// GET /categories
func (h *CategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.BusinessIDFromContext(r.Context())
	cats, err := h.repo.List(r.Context(), businessID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not list categories", nil)
		return
	}
	WriteJSON(w, http.StatusOK, cats)
}

// Create adds a new category for the caller's business.
// POST /categories
func (h *CategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CategoryRequest
	if err := ReadJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "datos inválidos", map[string]string{"name": "requerido"})
		return
	}

	businessID := middleware.BusinessIDFromContext(r.Context())
	cat := &model.Category{
		BusinessID:    businessID,
		Name:          req.Name,
		AllowMultiple: req.AllowMultiple,
	}
	if err := h.repo.Create(r.Context(), cat); err != nil {
		if isCategoryNameConflict(err) {
			WriteError(w, http.StatusConflict, categoryNameTakenMessage, nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "could not create category", nil)
		return
	}
	WriteJSON(w, http.StatusOK, cat)
}

// Update edits a category the caller owns.
// PUT /categories/{id}
func (h *CategoryHandler) Update(w http.ResponseWriter, r *http.Request) {
	current, ok := h.loadOwned(w, r)
	if !ok {
		return
	}

	var req model.CategoryRequest
	if err := ReadJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "datos inválidos", map[string]string{"name": "requerido"})
		return
	}

	current.Name = req.Name
	current.AllowMultiple = req.AllowMultiple
	if err := h.repo.Update(r.Context(), current.ID, current); err != nil {
		if isCategoryNameConflict(err) {
			WriteError(w, http.StatusConflict, categoryNameTakenMessage, nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "could not update category", nil)
		return
	}

	updated, err := h.repo.GetByID(r.Context(), current.ID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not reload category", nil)
		return
	}
	WriteJSON(w, http.StatusOK, updated)
}

// Delete removes a category the caller owns. Postgres FK from services
// without ON DELETE will reject this if any service still references it —
// surface that as 409 so the UI can prompt the user to reassign first.
// DELETE /categories/{id}
func (h *CategoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	cat, ok := h.loadOwned(w, r)
	if !ok {
		return
	}
	if err := h.repo.Delete(r.Context(), cat.ID); err != nil {
		if isForeignKeyViolation(err) {
			WriteError(w, http.StatusConflict, "no se puede eliminar: hay servicios que usan esta categoría", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "could not delete category", nil)
		return
	}
	WriteNoContent(w)
}

// loadOwned reads the {id} URL param, fetches the category, and verifies
// it belongs to the caller's business. Writes the appropriate error and
// returns ok=false on failure.
func (h *CategoryHandler) loadOwned(w http.ResponseWriter, r *http.Request) (*model.Category, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "id inválido", nil)
		return nil, false
	}
	cat, err := h.loadByIDOwned(r.Context(), id, middleware.BusinessIDFromContext(r.Context()))
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			WriteError(w, http.StatusNotFound, "categoría no encontrada", nil)
		case errors.Is(err, errForbidden):
			WriteError(w, http.StatusForbidden, "no autorizado", nil)
		default:
			WriteError(w, http.StatusInternalServerError, "could not load category", nil)
		}
		return nil, false
	}
	return cat, true
}

func (h *CategoryHandler) loadByIDOwned(ctx context.Context, id, businessID uuid.UUID) (*model.Category, error) {
	cat, err := h.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cat.BusinessID != businessID {
		return nil, errForbidden
	}
	return cat, nil
}

func isCategoryNameConflict(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == pgErrUniqueViolation && pgErr.ConstraintName == "categories_business_id_name_key"
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23503"
}
