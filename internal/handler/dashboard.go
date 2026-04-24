package handler

import (
	"net/http"
	"strconv"

	"github.com/mossandoval/datil-api/internal/middleware"
	"github.com/mossandoval/datil-api/internal/model"
	"github.com/mossandoval/datil-api/internal/repository"
)

const (
	dashboardDefaultLimit = 10
	dashboardMaxLimit     = 50
)

type DashboardHandler struct {
	repo         repository.DashboardRepository
	businessRepo repository.BusinessRepository
}

func NewDashboardHandler(repo repository.DashboardRepository, businessRepo repository.BusinessRepository) *DashboardHandler {
	return &DashboardHandler{repo: repo, businessRepo: businessRepo}
}

// Get returns dashboard data: today's count, week count, monthly income,
// upcoming, latest. Time boundaries are anchored to the business's timezone.
// GET /dashboard?upcoming_limit=N&latest_limit=N
func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	businessID := middleware.BusinessIDFromContext(ctx)

	business, err := h.businessRepo.GetByID(ctx, businessID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not load business", nil)
		return
	}
	loc := model.BusinessLocation(business.Timezone)

	upLim := parseDashboardLimit(r.URL.Query().Get("upcoming_limit"))
	latLim := parseDashboardLimit(r.URL.Query().Get("latest_limit"))

	data, err := h.repo.GetDashboard(ctx, userID, businessID, loc, upLim, latLim)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not load dashboard", nil)
		return
	}
	WriteJSON(w, http.StatusOK, data)
}

// parseDashboardLimit clamps to [1, dashboardMaxLimit]; empty/invalid ->
// default. Dashboard params are noise-tolerant — no 400s for out-of-range.
func parseDashboardLimit(raw string) int {
	if raw == "" {
		return dashboardDefaultLimit
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return dashboardDefaultLimit
	}
	if n > dashboardMaxLimit {
		return dashboardMaxLimit
	}
	return n
}
