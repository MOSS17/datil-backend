package handler

import (
	"net/http"

	"github.com/mossandoval/datil-api/internal/repository"
)

type DashboardHandler struct {
	repo repository.DashboardRepository
}

func NewDashboardHandler(repo repository.DashboardRepository) *DashboardHandler {
	return &DashboardHandler{repo: repo}
}

// Get returns dashboard data: today's count, week count, monthly income, upcoming, latest.
// GET /dashboard
func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}
