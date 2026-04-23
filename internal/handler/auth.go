package handler

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mossandoval/datil-api/internal/config"
	"github.com/mossandoval/datil-api/internal/repository"
)

type AuthHandler struct {
	userRepo     repository.UserRepository
	businessRepo repository.BusinessRepository
	pool         *pgxpool.Pool
	cfg          *config.Config
}

func NewAuthHandler(userRepo repository.UserRepository, businessRepo repository.BusinessRepository, pool *pgxpool.Pool, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		userRepo:     userRepo,
		businessRepo: businessRepo,
		pool:         pool,
		cfg:          cfg,
	}
}

// Signup creates a new business and user in a single transaction.
// POST /auth/signup
func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	// TODO: implement
	// 1. Parse SignupRequest from body
	// 2. Hash password with bcrypt
	// 3. Begin transaction with repository.WithTransaction
	// 4. Create business within tx
	// 5. Create user with business_id within tx
	// 6. Commit transaction
	// 7. Generate token pair
	// 8. Return AuthResponse
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}

// Login authenticates a user and returns a JWT token pair.
// POST /auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	// TODO: implement
	// 1. Parse LoginRequest from body
	// 2. Look up user by email
	// 3. Compare password with bcrypt
	// 4. Generate token pair
	// 5. Return AuthResponse
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}

// Refresh issues a new token pair given a valid refresh token.
// POST /auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	// TODO: implement
	// 1. Parse RefreshRequest from body
	// 2. Validate refresh token and extract claims
	// 3. Verify token type is "refresh"
	// 4. Generate new token pair
	// 5. Return AuthResponse
	WriteError(w, http.StatusNotImplemented, "not implemented", nil)
}
