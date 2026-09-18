package config

import "github.com/rodentskiedev/go-libraries/lib/env"

type Config struct {
	Port        string
	DatabaseURL string
}

func LoadConfig() Config {
	return Config{
		Port:        env.GetEnv("PORT", "8080"),
		DatabaseURL: env.GetEnv("DATABASE_URL", ""),
	}
}
