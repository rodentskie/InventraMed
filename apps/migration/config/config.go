package config

import "github.com/rodentskiedev/go-libraries/lib/env"

type Config struct {
	DatabaseURL string
	Action      string
}

func LoadConfig() Config {
	return Config{
		DatabaseURL: env.GetEnv("DATABASE_URL", ""),
		Action:      env.GetEnv("ACTION", "up"),
	}
}
