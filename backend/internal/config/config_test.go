package config

import "testing"

func TestLoadDefaultsPort(t *testing.T) {
	t.Setenv("APP_PORT", "")
	t.Setenv("DATABASE_URL", "postgres://example.test/database")
	t.Setenv("JWT_SECRET", "test-secret-with-at-least-thirty-two-characters")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != "8080" {
		t.Fatalf("Port = %q, want 8080", cfg.Port)
	}
}

func TestLoadRejectsAddress(t *testing.T) {
	t.Setenv("APP_PORT", ":8080")
	t.Setenv("DATABASE_URL", "postgres://example.test/database")
	t.Setenv("JWT_SECRET", "test-secret-with-at-least-thirty-two-characters")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid port error")
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "test-secret-with-at-least-thirty-two-characters")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing database URL error")
	}
}

func TestLoadRequiresStrongJWTSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example.test/database")
	t.Setenv("JWT_SECRET", "short")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want short JWT secret error")
	}
}
