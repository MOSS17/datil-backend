package handler

import (
	"errors"
	"fmt"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mossandoval/datil-api/internal/middleware"
	"github.com/mossandoval/datil-api/internal/model"
	"github.com/mossandoval/datil-api/internal/repository"
)

type ScheduleHandler struct {
	repo repository.ScheduleRepository
}

func NewScheduleHandler(repo repository.ScheduleRepository) *ScheduleHandler {
	return &ScheduleHandler{repo: repo}
}

// GetWorkdays returns all 7 workdays (Sun=0..Sat=6). Days the owner hasn't
// configured yet are returned as disabled placeholders with a nil ID and
// empty Hours slice, so the UI always renders a full week.
// GET /schedule/workdays
func (h *ScheduleHandler) GetWorkdays(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	businessID := middleware.BusinessIDFromContext(ctx)

	persisted, err := h.repo.ListWorkdays(ctx, businessID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not load workdays", nil)
		return
	}

	WriteJSON(w, http.StatusOK, fillMissingWorkdays(persisted, businessID))
}

// UpdateWorkdays replaces the whole-week schedule in one call. Body is the
// full []Workday array; partial updates aren't supported.
// PUT /schedule/workdays
func (h *ScheduleHandler) UpdateWorkdays(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	businessID := middleware.BusinessIDFromContext(ctx)

	var days []model.Workday
	if err := ReadJSON(w, r, &days); err != nil {
		WriteError(w, http.StatusBadRequest, "datos inválidos", nil)
		return
	}
	if errs := validateWorkdays(days); len(errs) > 0 {
		WriteError(w, http.StatusBadRequest, "datos inválidos", errs)
		return
	}

	if err := h.repo.UpsertWorkdays(ctx, businessID, days); err != nil {
		WriteError(w, http.StatusInternalServerError, "could not save workdays", nil)
		return
	}

	// Reload so the response carries canonical IDs + timestamps from the DB.
	persisted, err := h.repo.ListWorkdays(ctx, businessID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not reload workdays", nil)
		return
	}
	WriteJSON(w, http.StatusOK, fillMissingWorkdays(persisted, businessID))
}

// ListPersonalTime returns the caller's personal time blocks.
// GET /schedule/personal-time
func (h *ScheduleHandler) ListPersonalTime(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)

	out, err := h.repo.ListPersonalTime(ctx, userID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not load personal time", nil)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}

// CreatePersonalTime accepts the frontend's three UI shapes (hours/full_day/
// date_range) and normalises to the DB's (start_date, end_date, start_time?,
// end_time?). type/reason/date metadata is accepted but not persisted today.
// POST /schedule/personal-time
func (h *ScheduleHandler) CreatePersonalTime(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)

	var req model.CreatePersonalTimeRequest
	if err := ReadJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "datos inválidos", nil)
		return
	}

	startDate := req.StartDate
	endDate := req.EndDate
	if req.Date != "" {
		// "hours" and "full_day" shapes send a single `date` instead of a range.
		if startDate == "" {
			startDate = req.Date
		}
		if endDate == "" {
			endDate = req.Date
		}
	}

	fields := map[string]string{}
	if startDate == "" {
		fields["start_date"] = "requerido"
	}
	if endDate == "" {
		fields["end_date"] = "requerido"
	}
	if startDate != "" && endDate != "" && endDate < startDate {
		fields["end_date"] = "debe ser ≥ start_date"
	}
	// Mirror the DB CHECK: start_time/end_time must be paired, same-day, ordered.
	startTimeSet := req.StartTime != nil && *req.StartTime != ""
	endTimeSet := req.EndTime != nil && *req.EndTime != ""
	if startTimeSet != endTimeSet {
		fields["start_time"] = "start_time y end_time deben ir juntos"
	}
	if startTimeSet && endTimeSet {
		if startDate != endDate {
			fields["end_date"] = "con horas, start_date debe ser igual a end_date"
		}
		if *req.StartTime >= *req.EndTime {
			fields["end_time"] = "debe ser posterior a start_time"
		}
	}
	if len(fields) > 0 {
		WriteError(w, http.StatusBadRequest, "datos inválidos", fields)
		return
	}

	pt := &model.PersonalTime{
		UserID:    userID,
		StartDate: startDate,
		EndDate:   endDate,
	}
	if startTimeSet {
		pt.StartTime = req.StartTime
		pt.EndTime = req.EndTime
	}

	if err := h.repo.CreatePersonalTime(ctx, pt); err != nil {
		WriteError(w, http.StatusInternalServerError, "could not create personal time", nil)
		return
	}
	WriteJSON(w, http.StatusCreated, pt)
}

// DeletePersonalTime removes a block iff it belongs to the caller.
// DELETE /schedule/personal-time/{id}
func (h *ScheduleHandler) DeletePersonalTime(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "datos inválidos", map[string]string{"id": "uuid inválido"})
		return
	}

	pt, err := h.repo.GetPersonalTime(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "bloque no encontrado", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "could not load personal time", nil)
		return
	}
	if pt.UserID != userID {
		WriteError(w, http.StatusForbidden, "no autorizado", nil)
		return
	}

	if err := h.repo.DeletePersonalTime(ctx, id); err != nil {
		WriteError(w, http.StatusInternalServerError, "could not delete personal time", nil)
		return
	}
	WriteNoContent(w)
}

// --- helpers -----------------------------------------------------------------

// fillMissingWorkdays returns a length-7 slice covering Sun..Sat. Days the
// repo returned are preserved; gaps are filled with disabled placeholders.
// The UI depends on always getting 7 rows so the week renders consistently.
func fillMissingWorkdays(persisted []model.Workday, businessID uuid.UUID) []model.Workday {
	byDay := make(map[int]model.Workday, len(persisted))
	for _, d := range persisted {
		if d.Hours == nil {
			d.Hours = []model.WorkHour{}
		}
		byDay[d.Day] = d
	}
	out := make([]model.Workday, 0, 7)
	for day := 0; day < 7; day++ {
		if d, ok := byDay[day]; ok {
			out = append(out, d)
			continue
		}
		out = append(out, model.Workday{
			BusinessID: businessID,
			Day:        day,
			IsEnabled:  false,
			Hours:      []model.WorkHour{},
		})
	}
	return out
}

// validateWorkdays enforces: day ∈ [0..6], no duplicate days, each WorkHour
// has start_time < end_time, and WorkHours within a day don't overlap.
// Returns a field-keyed error map; empty map means valid.
func validateWorkdays(days []model.Workday) map[string]string {
	errs := map[string]string{}
	seenDay := map[int]bool{}
	for i, d := range days {
		if d.Day < 0 || d.Day > 6 {
			errs[fmt.Sprintf("[%d].day", i)] = "debe estar en 0..6"
			continue
		}
		if seenDay[d.Day] {
			errs[fmt.Sprintf("[%d].day", i)] = "día duplicado"
			continue
		}
		seenDay[d.Day] = true

		// Validate each hour, then sort + overlap-check.
		hours := append([]model.WorkHour(nil), d.Hours...)
		for j, h := range hours {
			if h.StartTime == "" || h.EndTime == "" {
				errs[fmt.Sprintf("[%d].hours[%d]", i, j)] = "start_time y end_time requeridos"
				continue
			}
			if h.StartTime >= h.EndTime {
				errs[fmt.Sprintf("[%d].hours[%d]", i, j)] = "end_time debe ser posterior a start_time"
			}
		}
		sort.Slice(hours, func(a, b int) bool { return hours[a].StartTime < hours[b].StartTime })
		for j := 1; j < len(hours); j++ {
			if hours[j].StartTime < hours[j-1].EndTime {
				errs[fmt.Sprintf("[%d].hours[%d]", i, j)] = "se traslapa con una hora anterior"
			}
		}
	}
	return errs
}
