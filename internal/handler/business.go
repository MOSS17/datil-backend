package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/mossandoval/datil-api/internal/middleware"
	"github.com/mossandoval/datil-api/internal/model"
	"github.com/mossandoval/datil-api/internal/repository"
	"github.com/mossandoval/datil-api/internal/storage"
)

type BusinessHandler struct {
	repo     repository.BusinessRepository
	uploader storage.Uploader
}

func NewBusinessHandler(repo repository.BusinessRepository, uploader storage.Uploader) *BusinessHandler {
	return &BusinessHandler{repo: repo, uploader: uploader}
}

const (
	maxLogoBytes  = int64(2 << 20)
	maxLogoMemory = int64(1 << 20)
)

var allowedLogoTypes = []string{"image/png", "image/jpeg", "image/webp"}

// Get returns the current user's business.
// GET /business
func (h *BusinessHandler) Get(w http.ResponseWriter, r *http.Request) {
	businessID := middleware.BusinessIDFromContext(r.Context())
	b, err := h.repo.GetByID(r.Context(), businessID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "negocio no encontrado", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "could not load business", nil)
		return
	}
	WriteJSON(w, http.StatusOK, b)
}

// Update updates the current user's business name, location, description.
// PUT /business
func (h *BusinessHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateBusinessRequest
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
	b := &model.Business{Name: req.Name, Location: req.Location, Description: req.Description}
	if err := h.repo.Update(r.Context(), businessID, b); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "negocio no encontrado", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "could not update business", nil)
		return
	}

	updated, err := h.repo.GetByID(r.Context(), businessID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not reload business", nil)
		return
	}
	WriteJSON(w, http.StatusOK, updated)
}

// UpdateBank sets all three bank fields atomically (the migration's check
// constraint requires they be set together).
// PUT /business/bank
func (h *BusinessHandler) UpdateBank(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateBankRequest
	if err := ReadJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", nil)
		return
	}

	req.BeneficiaryClabe = strings.TrimSpace(req.BeneficiaryClabe)
	req.BankName = strings.TrimSpace(req.BankName)
	req.BeneficiaryName = strings.TrimSpace(req.BeneficiaryName)

	fields := map[string]string{}
	if req.BeneficiaryClabe == "" {
		fields["beneficiary_clabe"] = "requerido"
	}
	if req.BankName == "" {
		fields["bank_name"] = "requerido"
	}
	if req.BeneficiaryName == "" {
		fields["beneficiary_name"] = "requerido"
	}
	if len(fields) > 0 {
		WriteError(w, http.StatusBadRequest, "datos inválidos", fields)
		return
	}

	businessID := middleware.BusinessIDFromContext(r.Context())
	if err := h.repo.UpdateBank(r.Context(), businessID, req.BeneficiaryClabe, req.BankName, req.BeneficiaryName); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "negocio no encontrado", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "could not update bank", nil)
		return
	}

	updated, err := h.repo.GetByID(r.Context(), businessID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not reload business", nil)
		return
	}
	WriteJSON(w, http.StatusOK, updated)
}

// UpdateLogo accepts multipart/form-data with field "logo", validates the
// content type via magic-byte sniffing, uploads to storage, and persists the URL.
// PUT /business/logo
func (h *BusinessHandler) UpdateLogo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLogoBytes+512)

	if err := r.ParseMultipartForm(maxLogoMemory); err != nil {
		WriteError(w, http.StatusBadRequest, "no se pudo leer el archivo (máx 2 MB)", nil)
		return
	}

	file, _, err := r.FormFile("logo")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "campo 'logo' requerido", nil)
		return
	}
	defer file.Close()

	contentType, body, err := storage.DetectAndValidate(file, allowedLogoTypes, maxLogoBytes)
	if err != nil {
		if errors.Is(err, storage.ErrInvalidContentType) {
			WriteError(w, http.StatusBadRequest, "formato no permitido (PNG, JPEG, WebP)", nil)
			return
		}
		WriteError(w, http.StatusBadRequest, "no se pudo leer el archivo", nil)
		return
	}

	businessID := middleware.BusinessIDFromContext(r.Context())
	key := fmt.Sprintf("businesses/%s/logo-%s", businessID, uuid.NewString())

	url, err := h.uploader.Upload(r.Context(), key, contentType, 0, body)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "no se pudo subir el archivo", nil)
		return
	}

	if err := h.repo.UpdateLogo(r.Context(), businessID, url); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "negocio no encontrado", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "could not save logo url", nil)
		return
	}

	updated, err := h.repo.GetByID(r.Context(), businessID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not reload business", nil)
		return
	}
	WriteJSON(w, http.StatusOK, updated)
}
