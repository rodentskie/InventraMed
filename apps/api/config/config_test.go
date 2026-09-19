package config

import (
	"testing"
	"time"
)

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", "secret")
}

func TestLoadConfig_Defaults(t *testing.T) {
	setRequired(t)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("Port: got %q, want 8080", cfg.Port)
	}
	if cfg.APIPrefix != "/api" {
		t.Errorf("APIPrefix: got %q, want /api", cfg.APIPrefix)
	}
	if cfg.JWTSecret != "secret" {
		t.Errorf("JWTSecret: got %q, want secret", cfg.JWTSecret)
	}
	if cfg.JWTAccessExpiry != 15*time.Minute {
		t.Errorf("JWTAccessExpiry: got %v, want 15m", cfg.JWTAccessExpiry)
	}
	if cfg.JWTRefreshExpiry != 168*time.Hour {
		t.Errorf("JWTRefreshExpiry: got %v, want 168h", cfg.JWTRefreshExpiry)
	}
}

func TestLoadConfig_Overrides(t *testing.T) {
	setRequired(t)
	t.Setenv("PORT", "9000")
	t.Setenv("API_PREFIX", "api/v1/")
	t.Setenv("JWT_ACCESS_EXPIRY", "1h")
	t.Setenv("JWT_REFRESH_EXPIRY", "24h")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Port != "9000" {
		t.Errorf("Port: got %q, want 9000", cfg.Port)
	}
	if cfg.APIPrefix != "/api/v1" {
		t.Errorf("APIPrefix: got %q, want /api/v1", cfg.APIPrefix)
	}
	if cfg.JWTAccessExpiry != time.Hour {
		t.Errorf("JWTAccessExpiry: got %v, want 1h", cfg.JWTAccessExpiry)
	}
	if cfg.JWTRefreshExpiry != 24*time.Hour {
		t.Errorf("JWTRefreshExpiry: got %v, want 24h", cfg.JWTRefreshExpiry)
	}
}

func TestLoadConfig_MissingSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected error for missing JWT_SECRET, got nil")
	}
}

func TestLoadConfig_InvalidAccessExpiry(t *testing.T) {
	setRequired(t)
	t.Setenv("JWT_ACCESS_EXPIRY", "soon")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected error for invalid JWT_ACCESS_EXPIRY, got nil")
	}
}

func TestLoadConfig_InvalidRefreshExpiry(t *testing.T) {
	setRequired(t)
	t.Setenv("JWT_REFRESH_EXPIRY", "later")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected error for invalid JWT_REFRESH_EXPIRY, got nil")
	}
}

func TestNormalizePrefix(t *testing.T) {
	tests := map[string]string{
		"/api":     "/api",
		"api":      "/api",
		"/api/":    "/api",
		" /api/ ":  "/api",
		"/api/v1/": "/api/v1",
		"":         "",
		"/":        "",
	}

	for in, want := range tests {
		if got := normalizePrefix(in); got != want {
			t.Errorf("normalizePrefix(%q): got %q, want %q", in, got, want)
		}
	}
}
