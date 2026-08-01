package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                   string
	DatabaseURL            string
	ReadTimeout            time.Duration
	WriteTimeout           time.Duration
	IdleTimeout            time.Duration
	GoogleOAuthClientID    string
	GoogleOAuthSecret      string
	GoogleOAuthRedirectURL string
	AuthSuccessRedirectURL string
	SessionTTL             time.Duration
	CookieSecure           bool
	DevHeaderAuth          bool
	AWSDefaultRegion       string
	AWSEndpointURL         string
	JudgeStorageBucket     string
	JudgeSubmissionsQueue  string
}

func Load() (Config, error) {
	cfg := Config{
		Port:                   getEnv("PORT", "8080"),
		DatabaseURL:            os.Getenv("DATABASE_URL"),
		ReadTimeout:            getDurationEnv("READ_TIMEOUT", 5*time.Second),
		WriteTimeout:           getDurationEnv("WRITE_TIMEOUT", 10*time.Second),
		IdleTimeout:            getDurationEnv("IDLE_TIMEOUT", 60*time.Second),
		GoogleOAuthClientID:    os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
		GoogleOAuthSecret:      os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"),
		GoogleOAuthRedirectURL: os.Getenv("GOOGLE_OAUTH_REDIRECT_URL"),
		AuthSuccessRedirectURL: getEnv("AUTH_SUCCESS_REDIRECT_URL", "/"),
		SessionTTL:             getDurationEnv("SESSION_TTL_SECONDS", 7*24*time.Hour),
		CookieSecure:           getBoolEnv("COOKIE_SECURE", false),
		DevHeaderAuth:          getBoolEnv("DEV_HEADER_AUTH", false),
		AWSDefaultRegion:       getEnv("AWS_DEFAULT_REGION", "ap-northeast-1"),
		AWSEndpointURL:         os.Getenv("AWS_ENDPOINT_URL"),
		JudgeStorageBucket:     getEnv("JUDGE_STORAGE_BUCKET", "judge-storage"),
		JudgeSubmissionsQueue:  getEnv("JUDGE_SUBMISSIONS_QUEUE_NAME", "judge-submissions"),
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

func getBoolEnv(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
