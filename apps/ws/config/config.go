package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/rodentskiedev/go-libraries/lib/env"
)

type Config struct {
	Port            string
	AllowedOrigins  []string
	ShutdownTimeout time.Duration
}

// LoadConfig reads the configuration from ENV. It returns an error when
// WS_SHUTDOWN_TIMEOUT cannot be parsed.
func LoadConfig() (Config, error) {
	shutdownTimeout, err := time.ParseDuration(env.GetEnv("WS_SHUTDOWN_TIMEOUT", "10s"))
	if err != nil {
		return Config{}, fmt.Errorf("parse WS_SHUTDOWN_TIMEOUT: %w", err)
	}

	return Config{
		Port:            env.GetEnv("PORT", "8081"),
		AllowedOrigins:  parseOrigins(env.GetEnv("WS_ALLOWED_ORIGINS", "")),
		ShutdownTimeout: shutdownTimeout,
	}, nil
}

// parseOrigins splits a comma-separated origin list, trimming whitespace and
// dropping empty entries.
func parseOrigins(raw string) []string {
	origins := []string{}
	for origin := range strings.SplitSeq(raw, ",") {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		origins = append(origins, origin)
	}

	return origins
}
