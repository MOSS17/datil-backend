package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/mossandoval/datil-api/internal/httpx"
)

type contextKey string

const (
	UserIDKey     contextKey = "userID"
	BusinessIDKey contextKey = "businessID"
)

type Claims struct {
	UserID     uuid.UUID `json:"user_id"`
	BusinessID uuid.UUID `json:"business_id"`
	TokenType  string    `json:"type"`
	jwt.RegisteredClaims
}

func JWTAuth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				httpx.WriteError(w, http.StatusUnauthorized, "missing authorization header", nil)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid authorization header format", nil)
				return
			}

			claims, err := parseToken(parts[1], jwtSecret)
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid or expired token", nil)
				return
			}

			if claims.TokenType == "refresh" {
				httpx.WriteError(w, http.StatusUnauthorized, "refresh tokens cannot be used for authentication", nil)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, BusinessIDKey, claims.BusinessID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func parseToken(tokenString, secret string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !token.Valid {
		if err == nil {
			err = fmt.Errorf("invalid token")
		}
		return nil, err
	}
	return claims, nil
}

// ParseRefreshToken parses a refresh token without rejecting it for being a refresh.
// Caller must check claims.TokenType == "refresh".
func ParseRefreshToken(tokenString, secret string) (*Claims, error) {
	return parseToken(tokenString, secret)
}

func UserIDFromContext(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(UserIDKey).(uuid.UUID)
	return id
}

func BusinessIDFromContext(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(BusinessIDKey).(uuid.UUID)
	return id
}

// GenerateTokenPair issues an (access, refresh) pair. The refresh token carries
// a unique JTI in RegisteredClaims.ID; the caller must persist it for rotation.
func GenerateTokenPair(userID, businessID uuid.UUID, secret string, accessExpiry, refreshExpiry time.Duration) (access, refresh string, refreshJTI uuid.UUID, err error) {
	now := time.Now()

	accessClaims := Claims{
		UserID:     userID,
		BusinessID: businessID,
		TokenType:  "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(accessExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	access, err = jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(secret))
	if err != nil {
		return "", "", uuid.Nil, fmt.Errorf("signing access token: %w", err)
	}

	refreshJTI = uuid.New()
	refreshClaims := Claims{
		UserID:     userID,
		BusinessID: businessID,
		TokenType:  "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        refreshJTI.String(),
			ExpiresAt: jwt.NewNumericDate(now.Add(refreshExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	refresh, err = jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString([]byte(secret))
	if err != nil {
		return "", "", uuid.Nil, fmt.Errorf("signing refresh token: %w", err)
	}

	return access, refresh, refreshJTI, nil
}
