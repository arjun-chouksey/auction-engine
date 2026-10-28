package config

import "testing"

func TestLoadDefaultsPort(t *testing.T) {
	t.Setenv("APP_PORT", "")
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
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid port error")
	}
}
