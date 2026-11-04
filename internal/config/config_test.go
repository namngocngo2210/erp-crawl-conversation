package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoad_Valid(t *testing.T) {
	t.Setenv("PANCAKE_ACCESS_TOKEN", "tok")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("REDIS_URL", "redis://x")
	t.Setenv("JWT_SECRET", "x")
	t.Setenv("WORKER_POOL_SIZE", "4")
	t.Setenv("RATE_LIMIT_PER_PAGE_RPS", "5")
	t.Setenv("TOKEN_TTL", "30m")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "tok", cfg.PancakeAccessToken)
	require.Equal(t, 4, cfg.WorkerPoolSize)
	require.Equal(t, 30*time.Minute, cfg.TokenTTL)
	require.Equal(t, "0 21 * * *", cfg.CronSchedule) // default
	require.Equal(t, "Asia/Ho_Chi_Minh", cfg.CronTimezone) // default
}

func TestLoad_MissingRequired(t *testing.T) {
	t.Setenv("PANCAKE_ACCESS_TOKEN", "")
	t.Setenv("DATABASE_URL", "")
	_, err := Load()
	require.Error(t, err)
}

func TestLoadReadAPIDefaults(t *testing.T) {
	t.Setenv("PANCAKE_ACCESS_TOKEN", "x")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("REDIS_URL", "redis://x")
	t.Setenv("JWT_SECRET", "secret")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JWTSecret != "secret" {
		t.Fatalf("want secret, got %q", cfg.JWTSecret)
	}
	if cfg.JWTTTL != 12*time.Hour {
		t.Fatalf("want 12h default, got %v", cfg.JWTTTL)
	}
	if cfg.CORSOrigin != "*" {
		t.Fatalf("want * default, got %q", cfg.CORSOrigin)
	}
}
