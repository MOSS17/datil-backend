package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/mossandoval/datil-api/internal/model"
	"github.com/mossandoval/datil-api/internal/repository"
)

// Shared multipart limits for endpoints that accept a payment-proof image
// (booking reserve + owner-side attach). Kept here because both the booking
// and appointment handlers need identical sizing/allowlist semantics — a
// divergence would let the frontend upload files the other endpoint rejects.
const (
	maxReserveBytes  = int64(6 << 20)
	maxReserveMemory = int64(2 << 20)
)

var allowedPaymentProofTypes = []string{
	"image/png", "image/jpeg", "image/webp", "application/pdf",
}

// Sentinel errors returned by resolveBookableServices; callers translate them
// to 400 responses with handler-specific Spanish copy.
var (
	errServiceNotFound     = errors.New("service not found")
	errServiceWrongOwner   = errors.New("service does not belong to business")
	errServiceNotExtra     = errors.New("service is not flagged as an extra")
)

// resolveBookableServices loads each requested service, verifies it belongs
// to the business, and returns the appointment_services rows, the ordered
// list of service names (for the WhatsApp body), the price total, and the
// duration total. Returns a sentinel error on the first invalid input so the
// caller can pick the right error response.
func resolveBookableServices(
	ctx context.Context,
	serviceRepo repository.ServiceRepository,
	businessID uuid.UUID,
	serviceIDs, extraIDs []uuid.UUID,
) (rows []model.AppointmentService, names []string, total float64, duration int, err error) {
	rows = make([]model.AppointmentService, 0, len(serviceIDs)+len(extraIDs))
	names = make([]string, 0, len(serviceIDs)+len(extraIDs))

	resolve := func(id uuid.UUID, requireExtra bool) error {
		s, rerr := serviceRepo.GetByID(ctx, id)
		if rerr != nil {
			return errServiceNotFound
		}
		if s.BusinessID != businessID {
			return errServiceWrongOwner
		}
		if requireExtra && !s.IsExtra {
			return errServiceNotExtra
		}
		rows = append(rows, model.AppointmentService{
			ServiceID: s.ID,
			Price:     s.MinPrice,
			Duration:  s.Duration,
		})
		names = append(names, s.Name)
		total += s.MinPrice
		duration += s.Duration
		return nil
	}

	for _, id := range serviceIDs {
		if err = resolve(id, false); err != nil {
			return nil, nil, 0, 0, err
		}
	}
	for _, id := range extraIDs {
		if err = resolve(id, true); err != nil {
			return nil, nil, 0, 0, err
		}
	}
	return rows, names, total, duration, nil
}

// writeBookableServiceError maps a resolveBookableServices sentinel to a 400
// with the right Spanish message. Falls through to 500 on unknown errors.
func writeBookableServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errServiceNotFound):
		WriteError(w, http.StatusBadRequest, "service_id inválido", nil)
	case errors.Is(err, errServiceWrongOwner):
		WriteError(w, http.StatusBadRequest, "service_id no pertenece a este negocio", nil)
	case errors.Is(err, errServiceNotExtra):
		WriteError(w, http.StatusBadRequest, "extra_id no es un servicio extra", nil)
	default:
		WriteError(w, http.StatusInternalServerError, "could not resolve services", nil)
	}
}
