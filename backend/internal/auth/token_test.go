package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/arjunchouksey/british-auction-rfq/backend/internal/domain"
)

func TestTokenIssueAndParse(t *testing.T) {
	manager, err := NewTokenManager("test-secret-with-at-least-thirty-two-characters", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	raw, err := manager.Issue(domain.User{ID: "user-id", Role: domain.RoleBuyer})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := manager.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if identity.UserID != "user-id" || identity.Role != domain.RoleBuyer {
		t.Fatalf("identity = %+v", identity)
	}

	manager.now = func() time.Time { return now.Add(2 * time.Hour) }
	if _, err := manager.Parse(raw); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expired Parse() error = %v, want ErrInvalidToken", err)
	}
}

func TestTokenManagerRejectsShortSecret(t *testing.T) {
	if _, err := NewTokenManager("short", time.Hour); err == nil {
		t.Fatal("NewTokenManager() error = nil, want short-secret error")
	}
}
