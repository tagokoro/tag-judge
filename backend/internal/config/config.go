package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port         string
	DatabaseURL  string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Port:         getEnv("PORT", "8080"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		ReadTimeout:  getDurationEnv("READ_TIMEOUT", 5*time.Second),
		WriteTimeout: getDurationEnv("WRITE_TIMEOUT", 10*time.Second),
		IdleTimeout:  getDurationEnv("IDLE_TIMEOUT", 60*time.Second),
	}

	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = fmt.Sprintf(
			"postgres://%s:%s@localhost:%s/%s?sslmode=disable",
			getEnv("POSTGRES_USER", "judge_user"),
			getEnv("POSTGRES_PASSWORD", "judge_pass"),
			getEnv("POSTGRES_PORT", "5432"),
			getEnv("POSTGRES_DB", "judge"),
		)
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}
