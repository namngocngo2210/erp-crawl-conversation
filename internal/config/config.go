package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	PancakeAccessToken    string
	PancakeBaseURL        string
	PancakeAccountBaseURL string
	DatabaseURL           string
	RedisURL              string
	CronSchedule          string
	CronTimezone          string
	HTTPAddr              string
	LogLevel              string
	MigrationsDir         string
	WorkerPoolSize        int
	RateLimitPerPageRPS   float64
	RateLimitPerPageBurst int
	TokenTTL              time.Duration
	JWTSecret             string
	AdminEmail            string
	AdminPassword         string
	CORSOrigin            string
	JWTTTL                time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		PancakeAccessToken:    os.Getenv("PANCAKE_ACCESS_TOKEN"),
		PancakeBaseURL:        getDefault("PANCAKE_BASE_URL", "https://pages.fm/api/public_api/v1"),
		PancakeAccountBaseURL: getDefault("PANCAKE_ACCOUNT_BASE_URL", "https://pages.fm/api/v1"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		RedisURL:              os.Getenv("REDIS_URL"),
		CronSchedule:          getDefault("CRON_SCHEDULE", "0 21 * * *"),
		CronTimezone:          getDefault("CRON_TIMEZONE", "Asia/Ho_Chi_Minh"),
		HTTPAddr:              getDefault("HTTP_ADDR", ":8080"),
		LogLevel:              getDefault("LOG_LEVEL", "info"),
		MigrationsDir:         getDefault("MIGRATIONS_DIR", "db/migrations"),
		WorkerPoolSize:        getInt("WORKER_POOL_SIZE", 8),
		RateLimitPerPageRPS:   getFloat("RATE_LIMIT_PER_PAGE_RPS", 5),
		RateLimitPerPageBurst: getInt("RATE_LIMIT_PER_PAGE_BURST", 5),
		TokenTTL:              getDuration("TOKEN_TTL", time.Hour),
		JWTSecret:             os.Getenv("JWT_SECRET"),
		AdminEmail:            getDefault("ADMIN_EMAIL", "admin@local"),
		AdminPassword:         os.Getenv("ADMIN_PASSWORD"),
		CORSOrigin:            getDefault("CORS_ORIGIN", "*"),
		JWTTTL:                getDuration("JWT_TTL", 12*time.Hour),
	}
	if cfg.PancakeAccessToken == "" {
		return Config{}, fmt.Errorf("PANCAKE_ACCESS_TOKEN is required")
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.RedisURL == "" {
		return Config{}, fmt.Errorf("REDIS_URL is required")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}
	return cfg, nil
}

func getDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

func getFloat(key string, def float64) float64 {
	if v, err := strconv.ParseFloat(os.Getenv(key), 64); err == nil {
		return v
	}
	return def
}

func getDuration(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(key)); err == nil {
		return v
	}
	return def
}
