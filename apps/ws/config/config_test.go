package config

import (
	"slices"
	"testing"
	"time"
)

func TestLoadConfig_Defaults(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Port != "8081" {
		t.Errorf("Port: got %q, want 8081", cfg.Port)
	}
	if len(cfg.AllowedOrigins) != 0 {
		t.Errorf("AllowedOrigins: got %v, want empty", cfg.AllowedOrigins)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("ShutdownTimeout: got %v, want 10s", cfg.ShutdownTimeout)
	}
}

func TestLoadConfig_Overrides(t *testing.T) {
	t.Setenv("PORT", "9001")
	t.Setenv("WS_ALLOWED_ORIGINS", "http://localhost:3000, https://inventramed.example")
	t.Setenv("WS_SHUTDOWN_TIMEOUT", "3s")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Port != "9001" {
		t.Errorf("Port: got %q, want 9001", cfg.Port)
	}
	want := []string{"http://localhost:3000", "https://inventramed.example"}
	if !slices.Equal(cfg.AllowedOrigins, want) {
		t.Errorf("AllowedOrigins: got %v, want %v", cfg.AllowedOrigins, want)
	}
	if cfg.ShutdownTimeout != 3*time.Second {
		t.Errorf("ShutdownTimeout: got %v, want 3s", cfg.ShutdownTimeout)
	}
}

func TestLoadConfig_InvalidShutdownTimeout(t *testing.T) {
	t.Setenv("WS_SHUTDOWN_TIMEOUT", "soon")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("LoadConfig: expected error for invalid WS_SHUTDOWN_TIMEOUT")
	}
}

func TestParseOrigins(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "empty", raw: "", want: []string{}},
		{name: "only separators", raw: " , ,", want: []string{}},
		{name: "wildcard", raw: "*", want: []string{"*"}},
		{name: "trims and drops empty", raw: " http://a.test ,,http://b.test ", want: []string{"http://a.test", "http://b.test"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseOrigins(tt.raw)
			if !slices.Equal(got, tt.want) {
				t.Errorf("parseOrigins(%q): got %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}
