package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort        string
	DatabaseURL    string
	JWTSecret      string
	TokenTTL       time.Duration
	IdempotencyTTL time.Duration
	AutoMigrate    bool
}

func Load() Config {
	_ = godotenv.Load()

	ttlHours, _ := strconv.Atoi(env("JWT_TTL_HOURS", "24"))
	autoMigrate, _ := strconv.ParseBool(env("AUTO_MIGRATE", "true"))
	return Config{
		AppPort:        env("APP_PORT", "8080"),
		DatabaseURL:    env("DATABASE_URL", "host=localhost user=postgres password=postgres dbname=task_manager port=5432 sslmode=disable TimeZone=Asia/Jakarta"),
		JWTSecret:      env("JWT_SECRET", "change-me-in-production"),
		TokenTTL:       time.Duration(ttlHours) * time.Hour,
		IdempotencyTTL: 24 * time.Hour,
		AutoMigrate:    autoMigrate,
	}
}

func (c Config) Addr() string { return fmt.Sprintf(":%s", c.AppPort) }

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
