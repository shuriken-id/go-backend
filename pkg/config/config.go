package config

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	Port       string
	DBURL      string
	JWTSecret  string
	TokenHours int
	GinMode    string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:       getenv("PORT", "8080"),
		DBURL:      os.Getenv("POSTGRESQL_DATABASE"),
		JWTSecret:  os.Getenv("JWT_SECRET"),
		TokenHours: getenvInt("TOKEN_HOURS", 24),
		GinMode:    getenv("GIN_MODE", "debug"),
	}
	if cfg.DBURL == "" {
		return nil, errors.New("POSTGRESQL_DATABASE is required")
	}
	if cfg.JWTSecret == "" {
		return nil, errors.New("JWT_SECRET is required")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}