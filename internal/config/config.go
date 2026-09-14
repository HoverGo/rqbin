package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config хранит настройки приложения из переменных окружения
type Config struct {
	Addr            string
	DatabaseURL     string
	PublicBaseURL   string
	RequestBodyLimit int64
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
}

// Load читает конфиг из env с разумными значениями по умолчанию
func Load() (Config, error) {
	cfg := Config{
		Addr:             env("ADDR", ":8080"),
		DatabaseURL:      env("DATABASE_URL", ""),
		PublicBaseURL:    env("PUBLIC_BASE_URL", "http://localhost:8080"),
		RequestBodyLimit: envInt64("REQUEST_BODY_LIMIT", 1<<20), // 1 MiB
		ReadTimeout:      envDuration("READ_TIMEOUT", 15*time.Second),
		WriteTimeout:     envDuration("WRITE_TIMEOUT", 15*time.Second),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt64(key string, fallback int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
