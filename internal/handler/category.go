package handler

import (
	"net/http"

	"github.com/mossandoval/datil-api/internal/repository"
)

type CategoryHandler struct {
	repo repository.CategoryRepository
}

func NewCategoryHandler(repo repository.CategoryRepository) *CategoryHandler {
	return &CategoryHandler{repo: repo}
}

// List returns all categories for the current user's business.
// GET /categories
func (h *CategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	ErrorJSON(w, http.StatusNotImplemented, "not implemented")
}

// Create creates a new category.
// POST /categories
func (h *CategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	ErrorJSON(w, http.StatusNotImplemented, "not implemented")
}

// Update updates a category.
// PUT /categories/{id}
func (h *CategoryHandler) Update(w http.ResponseWriter, r *http.Request) {
	ErrorJSON(w, http.StatusNotImplemented, "not implemented")
}

// Delete deletes a category.
// DELETE /categories/{id}
func (h *CategoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ErrorJSON(w, http.StatusNotImplemented, "not implemented")
}
