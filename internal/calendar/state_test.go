package calendar

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestStateSigner_RoundTrip(t *testing.T) {
	signer := NewStateSigner("test-secret")
	userID := uuid.New()

	tok, err := signer.Sign(userID)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	got, err := signer.Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got != userID {
		t.Fatalf("user id mismatch: got %s, want %s", got, userID)
	}
}

func TestStateSigner_RejectsWrongSecret(t *testing.T) {
	tok, err := NewStateSigner("secret-a").Sign(uuid.New())
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := NewStateSigner("secret-b").Verify(tok); err == nil {
		t.Fatal("expected verify with wrong secret to fail")
	}
}

func TestStateSigner_RejectsWrongType(t *testing.T) {
	secret := "test-secret"
	claims := stateClaims{
		UserID:    uuid.New(),
		TokenType: "access", // not "oauth_state"
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := NewStateSigner(secret).Verify(tok); err == nil {
		t.Fatal("expected verify to reject wrong token type")
	}
}

func TestStateSigner_RejectsExpired(t *testing.T) {
	secret := "test-secret"
	past := time.Now().Add(-1 * time.Hour)
	claims := stateClaims{
		UserID:    uuid.New(),
		TokenType: oauthStateType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(past),
			IssuedAt:  jwt.NewNumericDate(past.Add(-1 * time.Minute)),
		},
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := NewStateSigner(secret).Verify(tok); err == nil {
		t.Fatal("expected verify to reject expired token")
	}
}
