package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rodentskiedev/go-libraries/lib/env"
)

type Config struct {
	Port             string
	DatabaseURL      string
	APIPrefix        string
	JWTSecret        string
	JWTAccessExpiry  time.Duration
	JWTRefreshExpiry time.Duration
}

// LoadConfig reads the configuration from ENV. It returns an error when
// JWT_SECRET is missing or a duration cannot be parsed.
func LoadConfig() (Config, error) {
	secret := env.GetEnv("JWT_SECRET", "")
	if secret == "" {
		return Config{}, errors.New("JWT_SECRET is required")
	}

	accessExpiry, err := time.ParseDuration(env.GetEnv("JWT_ACCESS_EXPIRY", "15m"))
	if err != nil {
		return Config{}, fmt.Errorf("parse JWT_ACCESS_EXPIRY: %w", err)
	}

	refreshExpiry, err := time.ParseDuration(env.GetEnv("JWT_REFRESH_EXPIRY", "168h"))
	if err != nil {
		return Config{}, fmt.Errorf("parse JWT_REFRESH_EXPIRY: %w", err)
	}

	return Config{
		Port:             env.GetEnv("PORT", "8080"),
		DatabaseURL:      env.GetEnv("DATABASE_URL", ""),
		APIPrefix:        normalizePrefix(env.GetEnv("API_PREFIX", "/api")),
		JWTSecret:        secret,
		JWTAccessExpiry:  accessExpiry,
		JWTRefreshExpiry: refreshExpiry,
	}, nil
}

// normalizePrefix ensures a leading "/" and no trailing "/". An empty or
// slash-only value means no prefix.
func normalizePrefix(prefix string) string {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		return ""
	}

	return "/" + prefix
}
