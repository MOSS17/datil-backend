package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mossandoval/datil-api/internal/calendar"
	"github.com/mossandoval/datil-api/internal/config"
	"github.com/mossandoval/datil-api/internal/middleware"
	"github.com/mossandoval/datil-api/internal/model"
	"github.com/mossandoval/datil-api/internal/repository"
)

// feedLookbackWindow defines how far back we emit past appointments in the
// ICS feed. 30 days gives subscribers' Calendar apps a window to pick up
// recent cancellations and status transitions before the event drops off
// the feed. Longer windows grow the payload for no practical benefit.
const feedLookbackWindow = 30 * 24 * time.Hour

type CalendarHandler struct {
	cfg             *config.Config
	repo            repository.CalendarRepository
	userRepo        repository.UserRepository
	businessRepo    repository.BusinessRepository
	appointmentRepo repository.AppointmentRepository
	serviceRepo     repository.ServiceRepository
	google          *calendar.GoogleSyncer // nil when server isn't configured
	state           calendar.StateSigner
}

func NewCalendarHandler(
	cfg *config.Config,
	repo repository.CalendarRepository,
	userRepo repository.UserRepository,
	businessRepo repository.BusinessRepository,
	appointmentRepo repository.AppointmentRepository,
	serviceRepo repository.ServiceRepository,
	google *calendar.GoogleSyncer,
	state calendar.StateSigner,
) *CalendarHandler {
	return &CalendarHandler{
		cfg:             cfg,
		repo:            repo,
		userRepo:        userRepo,
		businessRepo:    businessRepo,
		appointmentRepo: appointmentRepo,
		serviceRepo:     serviceRepo,
		google:          google,
		state:           state,
	}
}

// Connect dispatches by {provider}. Google returns an authorize URL the
// frontend navigates to; ICS is credential-less — it just mints or
// returns the per-user feed token.
// POST /calendar/{provider}/connect
func (h *CalendarHandler) Connect(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	userID := middleware.UserIDFromContext(r.Context())

	switch provider {
	case "google":
		if h.google == nil {
			WriteError(w, http.StatusServiceUnavailable, "Google Calendar no está configurado en este servidor", nil)
			return
		}
		state, err := h.state.Sign(userID)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "no se pudo iniciar OAuth", nil)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{
			"authorize_url": h.google.AuthCodeURL(state),
		})
	case "ics":
		h.connectICS(w, r, userID)
	default:
		WriteError(w, http.StatusBadRequest, "provider no soportado", nil)
	}
}

// connectICS is idempotent: re-calling returns the existing webcal URL
// rather than minting a new token. A stable URL matters because calendar
// clients cache subscriptions indefinitely — reminting on every connect
// would silently break the subscription in the user's Apple Calendar.
func (h *CalendarHandler) connectICS(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	existing, err := h.repo.GetByUserAndProvider(r.Context(), userID, "ics")
	if err == nil && existing != nil && existing.FeedToken != nil {
		writeICSConnection(w, h.cfg.APIPublicBaseURL, *existing.FeedToken, existing.CreatedAt)
		return
	}
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		WriteError(w, http.StatusInternalServerError, "no se pudo leer la integración", nil)
		return
	}

	token, err := newFeedToken()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "no se pudo generar el token", nil)
		return
	}
	ci := &model.CalendarIntegration{
		UserID:    userID,
		Provider:  "ics",
		FeedToken: &token,
	}
	if err := h.repo.Upsert(r.Context(), ci); err != nil {
		WriteError(w, http.StatusInternalServerError, "no se pudo guardar la integración", nil)
		return
	}
	writeICSConnection(w, h.cfg.APIPublicBaseURL, token, ci.CreatedAt)
}

// RotateICS mints a fresh feed token. The previous URL stops working
// immediately — subscribers will see 404 and need the new URL. Used when
// a shared link leaks or an ex-employee had access.
// POST /calendar/ics/rotate
func (h *CalendarHandler) RotateICS(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	existing, err := h.repo.GetByUserAndProvider(r.Context(), userID, "ics")
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "no hay integración ICS que rotar", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "no se pudo leer la integración", nil)
		return
	}
	token, err := newFeedToken()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "no se pudo generar el token", nil)
		return
	}
	existing.FeedToken = &token
	if err := h.repo.Upsert(r.Context(), existing); err != nil {
		WriteError(w, http.StatusInternalServerError, "no se pudo actualizar la integración", nil)
		return
	}
	writeICSConnection(w, h.cfg.APIPublicBaseURL, token, existing.CreatedAt)
}

// Callback is Google-only. Apple has no redirect step. Google's redirect
// can't carry an Authorization header, so identity comes from the signed
// state parameter.
// GET /calendar/{provider}/callback
func (h *CalendarHandler) Callback(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	if provider != "google" {
		WriteError(w, http.StatusMethodNotAllowed, "callback solo soportado para google", nil)
		return
	}
	if h.google == nil {
		WriteError(w, http.StatusServiceUnavailable, "Google Calendar no está configurado en este servidor", nil)
		return
	}

	if errParam := r.URL.Query().Get("error"); errParam != "" {
		h.redirectFrontend(w, r, "google", errParam)
		return
	}

	code := r.URL.Query().Get("code")
	stateParam := r.URL.Query().Get("state")
	if code == "" || stateParam == "" {
		WriteError(w, http.StatusBadRequest, "faltan parámetros de OAuth", nil)
		return
	}

	userID, err := h.state.Verify(stateParam)
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "state inválido o expirado", nil)
		return
	}

	tok, email, err := h.google.Exchange(r.Context(), code)
	if err != nil {
		WriteError(w, http.StatusBadGateway, "no se pudo completar OAuth con Google", nil)
		return
	}

	accessToken := tok.AccessToken
	ci := &model.CalendarIntegration{
		UserID:      userID,
		Provider:    "google",
		AccessToken: &accessToken,
	}
	if tok.RefreshToken != "" {
		rt := tok.RefreshToken
		ci.RefreshToken = &rt
	}
	if !tok.Expiry.IsZero() {
		expiry := tok.Expiry
		ci.ExpiresAt = &expiry
	}
	if email != "" {
		ci.AccountEmail = &email
	}
	if err := h.repo.Upsert(r.Context(), ci); err != nil {
		WriteError(w, http.StatusInternalServerError, "no se pudo guardar la integración", nil)
		return
	}

	h.redirectFrontend(w, r, "google", "")
}

// Disconnect hard-deletes the integration row. Provider-agnostic — Google
// (OAuth) and ICS (feed token) both live in the same table with
// UNIQUE(user_id, provider).
// DELETE /calendar/{provider}
func (h *CalendarHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	if provider != "google" && provider != "ics" {
		WriteError(w, http.StatusBadRequest, "provider no soportado", nil)
		return
	}
	userID := middleware.UserIDFromContext(r.Context())
	if err := h.repo.Delete(r.Context(), userID, provider); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "integración no encontrada", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "no se pudo eliminar la integración", nil)
		return
	}
	WriteNoContent(w)
}

// ServeFeed is the public, unauthenticated endpoint that Calendar apps
// poll. Any error path returns 404 so we never leak whether a given
// token ever existed.
// GET /calendar/ics/{token}.ics   — registered OUTSIDE the /api/v1 prefix
func (h *CalendarHandler) ServeFeed(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSuffix(chi.URLParam(r, "token"), ".ics")
	if token == "" {
		http.NotFound(w, r)
		return
	}

	ci, err := h.repo.GetByFeedToken(r.Context(), token)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	user, err := h.userRepo.GetByID(r.Context(), ci.UserID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	business, err := h.businessRepo.GetByID(r.Context(), user.BusinessID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// 30-day lookback window: cancellations propagate to subscribers for a
	// month after the event, then drop off. Far-future upper bound keeps
	// the query boundless so owners' booked-out calendars come through.
	loc := model.BusinessLocation(business.Timezone)
	now := time.Now().In(loc)
	from := now.Add(-feedLookbackWindow)
	to := now.Add(365 * 24 * time.Hour)

	appts, err := h.appointmentRepo.List(r.Context(), user.ID, from, to)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	feedInput, err := h.buildFeedInput(r.Context(), *business, appts)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	payload, err := calendar.RenderFeed(feedInput)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("Content-Disposition", `inline; filename="datil.ics"`)
	_, _ = w.Write(payload)
}

// buildFeedInput enriches appointments with their service/extra names for
// the ICS SUMMARY and DESCRIPTION. Two queries total: appointment_services
// (batch) + services by business (single query, used as a lookup map).
func (h *CalendarHandler) buildFeedInput(ctx context.Context, business model.Business, appts []model.Appointment) (calendar.FeedInput, error) {
	apptIDs := make([]uuid.UUID, 0, len(appts))
	for _, a := range appts {
		apptIDs = append(apptIDs, a.ID)
	}
	servicesMap, err := h.appointmentRepo.ListServicesFor(ctx, apptIDs)
	if err != nil {
		return calendar.FeedInput{}, err
	}

	// Build a single Service lookup covering the business's catalog. All
	// appointment_services rows reference services owned by this business,
	// so one query suffices regardless of how many appointments are in the
	// feed window.
	services, err := h.serviceRepo.List(ctx, business.ID)
	if err != nil {
		return calendar.FeedInput{}, err
	}
	lookup := make(map[uuid.UUID]model.Service, len(services))
	for _, s := range services {
		lookup[s.ID] = s
	}

	feedAppts := make([]calendar.FeedAppointment, 0, len(appts))
	for _, a := range appts {
		fa := calendar.FeedAppointment{Appointment: a}
		for _, link := range servicesMap[a.ID] {
			svc, ok := lookup[link.ServiceID]
			if !ok {
				continue
			}
			if svc.IsExtra {
				fa.ExtraNames = append(fa.ExtraNames, svc.Name)
			} else {
				fa.ServiceNames = append(fa.ServiceNames, svc.Name)
			}
			fa.ServiceLines = append(fa.ServiceLines, fmt.Sprintf("%s — $%.0f", svc.Name, link.Price))
		}
		feedAppts = append(feedAppts, fa)
	}

	return calendar.FeedInput{
		Business:     business,
		Appointments: feedAppts,
	}, nil
}

// redirectFrontend sends the browser back to the frontend after OAuth.
// Errors are passed via ?error=... so the frontend can surface a toast.
// If FRONTEND_BASE_URL isn't configured we return 200 with a JSON body
// instead — safer than a half-broken 302.
func (h *CalendarHandler) redirectFrontend(w http.ResponseWriter, r *http.Request, provider, errVal string) {
	base := strings.TrimRight(h.cfg.FrontendBaseURL, "/")
	if base == "" {
		WriteJSON(w, http.StatusOK, map[string]string{"provider": provider, "status": "connected"})
		return
	}
	u, err := url.Parse(base + "/calendar")
	if err != nil {
		WriteJSON(w, http.StatusOK, map[string]string{"provider": provider, "status": "connected"})
		return
	}
	q := u.Query()
	if errVal == "" {
		q.Set("connected", provider)
	} else {
		q.Set("error", errVal)
		q.Set("provider", provider)
	}
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

// newFeedToken mints a URL-safe, padding-free token. 32 bytes → 43 chars
// base64url is roomy enough to resist brute-force enumeration even with
// the calendar feed's public, unauthenticated surface.
func newFeedToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating feed token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

type icsConnectionResponse struct {
	WebcalURL   string    `json:"webcal_url"`
	HTTPSURL    string    `json:"https_url"`
	ConnectedAt time.Time `json:"connected_at"`
}

func writeICSConnection(w http.ResponseWriter, apiBase, token string, connectedAt time.Time) {
	httpsURL := strings.TrimRight(apiBase, "/") + "/calendar/ics/" + token + ".ics"
	// webcal:// is the iCal subscription scheme. Swap only the scheme;
	// path stays the same so users can paste either form into clients
	// that don't recognise webcal://.
	webcalURL := httpsURL
	if idx := strings.Index(httpsURL, "://"); idx > 0 {
		webcalURL = "webcal" + httpsURL[idx:]
	}
	WriteJSON(w, http.StatusOK, icsConnectionResponse{
		WebcalURL:   webcalURL,
		HTTPSURL:    httpsURL,
		ConnectedAt: connectedAt,
	})
}
