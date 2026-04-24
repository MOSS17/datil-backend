package calendar

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// oauthStateType tags state JWTs so the main middleware.JWTAuth rejects
// them if one ever gets bolted onto an Authorization header. Keeps the
// blast radius small if a state token leaks (short TTL + wrong type).
const oauthStateType = "oauth_state"

// oauthStateTTL balances OAuth round-trip latency (seconds, plus user time
// on the consent screen) against replay-attack exposure. Google's consent
// screen rarely takes longer than a minute or two.
const oauthStateTTL = 5 * time.Minute

type stateClaims struct {
	UserID    uuid.UUID `json:"user_id"`
	TokenType string    `json:"type"`
	jwt.RegisteredClaims
}

// StateSigner signs and verifies the `state` parameter threaded through
// Google's OAuth redirect. It does two jobs at once: CSRF protection
// (Google only redirects the user back with a state we issued) and
// identity forwarding (the callback can't read the JWT in Authorization
// because it's a top-level browser redirect, so we stash user_id inside
// state).
type StateSigner struct {
	secret []byte
}

func NewStateSigner(secret string) StateSigner {
	return StateSigner{secret: []byte(secret)}
}

func (s StateSigner) Sign(userID uuid.UUID) (string, error) {
	now := time.Now()
	claims := stateClaims{
		UserID:    userID,
		TokenType: oauthStateType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(oauthStateTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

func (s StateSigner) Verify(tok string) (uuid.UUID, error) {
	claims := &stateClaims{}
	parsed, err := jwt.ParseWithClaims(tok, claims, func(*jwt.Token) (interface{}, error) {
		return s.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return uuid.Nil, err
	}
	if !parsed.Valid {
		return uuid.Nil, errors.New("invalid state token")
	}
	if claims.TokenType != oauthStateType {
		return uuid.Nil, fmt.Errorf("wrong state token type %q", claims.TokenType)
	}
	return claims.UserID, nil
}
