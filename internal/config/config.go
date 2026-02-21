package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration.
type Config struct {
	ServerPort     string
	GinMode        string
	LogLevel       string
	LogFormat      string
	DatabaseURL    string
	MaxDBConns     int32
	RedisURL       string
	JWTSecret      string
	JWTExpiry      time.Duration
	BcryptCost     int
	UploadDir      string
	MaxUploadBytes int64
	// AllowedOrigins controls HTTP CORS and WebSocket origin validation.
	// Empty slice means all origins are permitted (dev default).
	AllowedOrigins []string
}

// Load reads configuration from environment variables with sensible defaults.
// It loads .env file if present but does not fail if missing.
func Load() *Config {
	_ = godotenv.Load() // Ignore error — .env is optional

	return &Config{
		ServerPort:     getEnv("SERVER_PORT", "8080"),
		GinMode:        getEnv("GIN_MODE", "debug"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		LogFormat:      getEnv("LOG_FORMAT", "pretty"),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://exstem:exstem_secret@localhost:5432/exstem?sslmode=disable"),
		MaxDBConns:     int32(getEnvInt("MAX_DB_CONNS", 16)),
		RedisURL:       getEnv("REDIS_URL", "redis://localhost:6379/0"),
		JWTSecret:      getEnv("JWT_SECRET", "change-this-to-a-secure-random-string"),
		JWTExpiry:      time.Duration(getEnvInt("JWT_EXPIRY_HOURS", 24)) * time.Hour,
		BcryptCost:     getEnvInt("BCRYPT_COST", 6),
		UploadDir:      getEnv("UPLOAD_DIR", "./uploads"),
		MaxUploadBytes: int64(getEnvInt("MAX_UPLOAD_SIZE_MB", 10)) * 1024 * 1024,
		AllowedOrigins: parseOrigins(getEnv("ALLOWED_ORIGINS", "")),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// parseOrigins splits a comma-separated origins string into a trimmed slice.
// Returns nil (allow-all) if the input is empty.
func parseOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}
