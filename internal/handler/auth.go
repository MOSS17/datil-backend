package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mossandoval/datil-api/internal/config"
	"github.com/mossandoval/datil-api/internal/middleware"
	"github.com/mossandoval/datil-api/internal/model"
	"github.com/mossandoval/datil-api/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	userRepo     repository.UserRepository
	businessRepo repository.BusinessRepository
	refreshRepo  repository.RefreshTokenRepository
	pool         *pgxpool.Pool
	cfg          *config.Config
}

func NewAuthHandler(
	userRepo repository.UserRepository,
	businessRepo repository.BusinessRepository,
	refreshRepo repository.RefreshTokenRepository,
	pool *pgxpool.Pool,
	cfg *config.Config,
) *AuthHandler {
	return &AuthHandler{
		userRepo:     userRepo,
		businessRepo: businessRepo,
		refreshRepo:  refreshRepo,
		pool:         pool,
		cfg:          cfg,
	}
}

const (
	pgErrUniqueViolation = "23505"
	emailTakenMessage    = "Ese correo ya está registrado"
	invalidCredsMessage  = "Correo o contraseña incorrectos"
	maxSlugRetries       = 3
)

var emailRegex = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// Signup creates a business + user atomically and returns a token pair.
func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var req model.SignupRequest
	if err := ReadJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", nil)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.BusinessName = strings.TrimSpace(req.BusinessName)
	req.Timezone = strings.TrimSpace(req.Timezone)

	if fields := validateSignup(req); len(fields) > 0 {
		WriteError(w, http.StatusBadRequest, "datos inválidos", fields)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), h.cfg.BcryptCost)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not hash password", nil)
		return
	}

	user := &model.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: string(hash),
	}
	business := &model.Business{Name: req.BusinessName, Timezone: req.Timezone}

	emailTaken, err := h.createBusinessAndUser(r.Context(), business, user)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not create account", nil)
		return
	}
	if emailTaken {
		WriteError(w, http.StatusConflict, emailTakenMessage, nil)
		return
	}

	h.issueTokenPair(w, r, user)
}

// createBusinessAndUser inserts the business with a unique slug and the user in
// one transaction. Retries the slug a few times on collision.
// Returns emailTaken=true when the unique violation comes from users.email.
func (h *AuthHandler) createBusinessAndUser(ctx context.Context, b *model.Business, u *model.User) (bool, error) {
	baseSlug := slugify(b.Name)
	if baseSlug == "" {
		baseSlug = "business"
	}

	var emailTaken bool
	for range maxSlugRetries {
		b.URL = baseSlug + "-" + randomSuffix()
		err := repository.WithTransaction(ctx, h.pool, func(tx pgx.Tx) error {
			if err := h.businessRepo.Create(ctx, tx, b); err != nil {
				return err
			}
			u.BusinessID = b.ID
			return h.userRepo.Create(ctx, tx, u)
		})
		if err == nil {
			return false, nil
		}
		if isUniqueViolation(err, "users_email_key") {
			emailTaken = true
			return emailTaken, nil
		}
		if isUniqueViolation(err, "businesses_url_key") {
			continue
		}
		return false, err
	}
	return false, errors.New("could not generate unique business slug")
}

// Login authenticates with bcrypt and returns a token pair on success.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req model.LoginRequest
	if err := ReadJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", nil)
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		WriteError(w, http.StatusUnauthorized, invalidCredsMessage, nil)
		return
	}

	user, err := h.userRepo.GetByEmail(r.Context(), req.Email)
	if err != nil {
		WriteError(w, http.StatusUnauthorized, invalidCredsMessage, nil)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		WriteError(w, http.StatusUnauthorized, invalidCredsMessage, nil)
		return
	}

	h.issueTokenPair(w, r, user)
}

// Refresh consumes a refresh token, issues a new pair, and detects replay.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req model.RefreshRequest
	if err := ReadJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	if req.RefreshToken == "" {
		WriteError(w, http.StatusUnauthorized, "invalid refresh token", nil)
		return
	}

	claims, err := middleware.ParseRefreshToken(req.RefreshToken, h.cfg.JWTSecret)
	if err != nil || claims.TokenType != "refresh" {
		WriteError(w, http.StatusUnauthorized, "invalid refresh token", nil)
		return
	}

	jti, err := uuid.Parse(claims.ID)
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "invalid refresh token", nil)
		return
	}

	alreadyUsed, err := h.refreshRepo.MarkUsed(r.Context(), jti)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not refresh session", nil)
		return
	}
	if alreadyUsed {
		// Theft signal: a previously consumed token was presented again.
		// Burn every refresh token for this user — they must log in fresh.
		_ = h.refreshRepo.DeleteAllForUser(r.Context(), claims.UserID)
		WriteError(w, http.StatusUnauthorized, "refresh token reused", nil)
		return
	}

	user, err := h.userRepo.GetByID(r.Context(), claims.UserID)
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "invalid refresh token", nil)
		return
	}

	h.issueTokenPair(w, r, user)
}

func (h *AuthHandler) issueTokenPair(w http.ResponseWriter, r *http.Request, user *model.User) {
	access, refresh, jti, err := middleware.GenerateTokenPair(
		user.ID, user.BusinessID, h.cfg.JWTSecret,
		h.cfg.JWTAccessExpiry, h.cfg.JWTRefreshExpiry,
	)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not issue tokens", nil)
		return
	}

	expiresAt := time.Now().Add(h.cfg.JWTRefreshExpiry)
	if err := h.refreshRepo.Insert(r.Context(), jti, user.ID, expiresAt); err != nil {
		WriteError(w, http.StatusInternalServerError, "could not persist refresh token", nil)
		return
	}

	WriteJSON(w, http.StatusOK, model.AuthResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		User:         *user,
	})
}

func validateSignup(req model.SignupRequest) map[string]string {
	fields := map[string]string{}
	if req.Name == "" {
		fields["name"] = "requerido"
	}
	if req.BusinessName == "" {
		fields["business_name"] = "requerido"
	}
	if req.Email == "" {
		fields["email"] = "requerido"
	} else if !emailRegex.MatchString(req.Email) {
		fields["email"] = "formato inválido"
	}
	if len(req.Password) < 8 {
		fields["password"] = "mínimo 8 caracteres"
	}
	// Timezone is optional; validate only if the client sent one. Empty
	// string falls through to the repo default.
	if req.Timezone != "" {
		if _, err := time.LoadLocation(req.Timezone); err != nil {
			fields["timezone"] = "zona horaria inválida"
		}
	}
	return fields
}

var slugRegex = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRegex.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func randomSuffix() string {
	var b [2]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	if pgErr.Code != pgErrUniqueViolation {
		return false
	}
	if constraint == "" {
		return true
	}
	return pgErr.ConstraintName == constraint
}
