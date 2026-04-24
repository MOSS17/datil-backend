package handler

import (
	"errors"
	"net/http"
	"net/mail"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mossandoval/datil-api/internal/calendar"
	"github.com/mossandoval/datil-api/internal/config"
	"github.com/mossandoval/datil-api/internal/middleware"
	"github.com/mossandoval/datil-api/internal/model"
	"github.com/mossandoval/datil-api/internal/repository"
)

type CalendarHandler struct {
	cfg    *config.Config
	repo   repository.CalendarRepository
	google *calendar.GoogleSyncer // may be nil if server isn't configured
	apple  *calendar.AppleSyncer
	state  calendar.StateSigner
}

func NewCalendarHandler(
	cfg *config.Config,
	repo repository.CalendarRepository,
	google *calendar.GoogleSyncer,
	apple *calendar.AppleSyncer,
	state calendar.StateSigner,
) *CalendarHandler {
	return &CalendarHandler{
		cfg:    cfg,
		repo:   repo,
		google: google,
		apple:  apple,
		state:  state,
	}
}

// Connect dispatches on {provider}. Google returns an authorize URL the
// frontend navigates to; Apple accepts credentials inline and validates
// them before persisting. Shape difference is intentional — Apple doesn't
// do OAuth, so the flows diverge at the handler boundary, not deeper.
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
	case "apple":
		var req struct {
			Email       string `json:"email"`
			AppPassword string `json:"app_password"`
		}
		if err := ReadJSON(w, r, &req); err != nil {
			WriteError(w, http.StatusBadRequest, "datos inválidos", nil)
			return
		}
		req.Email = strings.TrimSpace(req.Email)
		req.AppPassword = strings.TrimSpace(req.AppPassword)

		fields := map[string]string{}
		if _, err := mail.ParseAddress(req.Email); err != nil {
			fields["email"] = "correo inválido"
		}
		if req.AppPassword == "" {
			fields["app_password"] = "requerido"
		}
		if len(fields) > 0 {
			WriteError(w, http.StatusBadRequest, "datos inválidos", fields)
			return
		}

		if err := h.apple.Validate(r.Context(), req.Email, req.AppPassword); err != nil {
			WriteError(w, http.StatusUnauthorized, "no se pudo autenticar con iCloud (verifica la contraseña de aplicación)", nil)
			return
		}

		ci := &model.CalendarIntegration{
			UserID:       userID,
			Provider:     "apple",
			AccessToken:  req.AppPassword,
			AccountEmail: &req.Email,
		}
		if err := h.repo.Upsert(r.Context(), ci); err != nil {
			WriteError(w, http.StatusInternalServerError, "no se pudo guardar la integración", nil)
			return
		}
		WriteJSON(w, http.StatusOK, ci)
	default:
		WriteError(w, http.StatusBadRequest, "provider no soportado", nil)
	}
}

// Callback is Google-only. Apple has no redirect step — its Connect is a
// direct POST. Google's redirect can't carry an Authorization header so we
// reconstruct identity from the signed state parameter.
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

	ci := &model.CalendarIntegration{
		UserID:      userID,
		Provider:    "google",
		AccessToken: tok.AccessToken,
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

// Disconnect removes an integration. Provider-agnostic — Google and Apple
// use the same table, same row shape.
// DELETE /calendar/{provider}
func (h *CalendarHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	if provider != "google" && provider != "apple" {
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
