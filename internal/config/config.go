package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv           string
	AppName          string
	HTTPPort         string
	DatabaseURL      string
	RedisURL         string
	JWTAccessSecret  string
	JWTRefreshSecret string
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration
	CORSOrigins      []string
	UploadDir        string
	MaxUploadBytes   int64
	PublicBaseURL    string
	RateLimitRPM     int
	// Calling (WebRTC)
	STUNURLs        []string
	TURNURLs        []string
	TURNUsername    string
	TURNCredential  string
	MaxParticipants int
	CallTimeoutSec  int
	// Web Push (VAPID)
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	VAPIDSubject    string
}

func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:           getEnv("APP_ENV", "development"),
		AppName:          getEnv("APP_NAME", "Pulse Chat Service"),
		HTTPPort:         getEnv("PORT", getEnv("HTTP_PORT", "8080")),
		DatabaseURL:      mustEnv("DATABASE_URL", "postgres://chat:chat@localhost:5432/chat?sslmode=disable"),
		RedisURL:         mustEnv("REDIS_URL", "redis://localhost:6379/0"),
		JWTAccessSecret:  os.Getenv("JWT_ACCESS_SECRET"),
		JWTRefreshSecret: os.Getenv("JWT_REFRESH_SECRET"),
		UploadDir:        getEnv("UPLOAD_DIR", "./uploads"),
		PublicBaseURL:    getEnv("PUBLIC_BASE_URL", "http://localhost:8080"),
		CORSOrigins:      splitCSV(getEnv("CORS_ORIGINS", "http://localhost:3000")),
		MaxUploadBytes:   int64(getEnvInt("MAX_UPLOAD_MB", 50)) * 1024 * 1024,
		RateLimitRPM:     getEnvInt("RATE_LIMIT_RPM", 120),
		STUNURLs:         splitCSV(getEnv("STUN_URLS", "stun:stun.l.google.com:19302")),
		TURNURLs:         splitCSV(getEnv("TURN_URLS", "")),
		TURNUsername:     getEnv("TURN_USERNAME", ""),
		TURNCredential:   getEnv("TURN_CREDENTIAL", ""),
		MaxParticipants:  getEnvInt("MAX_PARTICIPANTS", 12),
		CallTimeoutSec:   getEnvInt("CALL_TIMEOUT_SEC", 60),
		VAPIDPublicKey:   os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey:  os.Getenv("VAPID_PRIVATE_KEY"),
		VAPIDSubject:     getEnv("VAPID_SUBJECT", "mailto:admin@pulse.local"),
	}

	accessMin := getEnvInt("ACCESS_TOKEN_TTL_MIN", 15)
	refreshDays := getEnvInt("REFRESH_TOKEN_TTL_DAYS", 30)
	cfg.AccessTokenTTL = time.Duration(accessMin) * time.Minute
	cfg.RefreshTokenTTL = time.Duration(refreshDays) * 24 * time.Hour

	if cfg.AppEnv == "development" {
		if cfg.JWTAccessSecret == "" {
			cfg.JWTAccessSecret = "dev-only-access-secret-do-not-use-prod!!"
		}
		if cfg.JWTRefreshSecret == "" {
			cfg.JWTRefreshSecret = "dev-only-refresh-secret-do-not-use-prod!"
		}
	}

	if len(cfg.JWTAccessSecret) < 32 || len(cfg.JWTRefreshSecret) < 32 {
		return nil, fmt.Errorf("JWT_ACCESS_SECRET and JWT_REFRESH_SECRET must be set and at least 32 characters")
	}
	if cfg.AppEnv == "production" {
		if strings.Contains(cfg.JWTAccessSecret, "dev-only") || strings.Contains(cfg.JWTAccessSecret, "change-me") {
			return nil, fmt.Errorf("production requires strong JWT_ACCESS_SECRET")
		}
		if strings.Contains(cfg.JWTRefreshSecret, "dev-only") || strings.Contains(cfg.JWTRefreshSecret, "change-me") {
			return nil, fmt.Errorf("production requires strong JWT_REFRESH_SECRET")
		}
		if len(cfg.CORSOrigins) == 0 {
			return nil, fmt.Errorf("production requires CORS_ORIGINS")
		}
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustEnv(key, fallback string) string {
	return getEnv(key, fallback)
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

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
