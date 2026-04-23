package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port               string
	Env                string
	DatabaseURL        string
	JWTSecret          string
	JWTAccessExpiry    time.Duration
	JWTRefreshExpiry   time.Duration
	CORSAllowedOrigins []string
	TwilioAccountSID   string
	TwilioAuthToken    string
	TwilioWhatsAppFrom string
	BcryptCost         int

	// Storage
	StorageProvider   string // "local" | "r2"
	LocalUploadRoot   string
	LocalPublicBaseURL string
	R2AccountID       string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2Bucket          string
	R2PublicBaseURL   string
}

func Load() (*Config, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	accessExpiry, err := time.ParseDuration(getEnvOrDefault("JWT_ACCESS_EXPIRY", "15m"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_ACCESS_EXPIRY: %w", err)
	}

	refreshExpiry, err := time.ParseDuration(getEnvOrDefault("JWT_REFRESH_EXPIRY", "168h"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_REFRESH_EXPIRY: %w", err)
	}

	origins := getEnvOrDefault("CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	allowedOrigins := strings.Split(origins, ",")
	for i := range allowedOrigins {
		allowedOrigins[i] = strings.TrimSpace(allowedOrigins[i])
	}

	bcryptCost, err := strconv.Atoi(getEnvOrDefault("BCRYPT_COST", "12"))
	if err != nil {
		return nil, fmt.Errorf("invalid BCRYPT_COST: %w", err)
	}

	provider := getEnvOrDefault("STORAGE_PROVIDER", "local")
	cfg := &Config{
		Port:               getEnvOrDefault("PORT", "8080"),
		Env:                getEnvOrDefault("ENV", "development"),
		DatabaseURL:        databaseURL,
		JWTSecret:          jwtSecret,
		JWTAccessExpiry:    accessExpiry,
		JWTRefreshExpiry:   refreshExpiry,
		CORSAllowedOrigins: allowedOrigins,
		TwilioAccountSID:   os.Getenv("TWILIO_ACCOUNT_SID"),
		TwilioAuthToken:    os.Getenv("TWILIO_AUTH_TOKEN"),
		TwilioWhatsAppFrom: os.Getenv("TWILIO_WHATSAPP_FROM"),
		BcryptCost:         bcryptCost,
		StorageProvider:    provider,
		LocalUploadRoot:    getEnvOrDefault("LOCAL_UPLOAD_ROOT", "./.uploads"),
		LocalPublicBaseURL: getEnvOrDefault("LOCAL_PUBLIC_BASE_URL", "http://localhost:8080/static/uploads"),
		R2AccountID:        os.Getenv("R2_ACCOUNT_ID"),
		R2AccessKeyID:      os.Getenv("R2_ACCESS_KEY_ID"),
		R2SecretAccessKey:  os.Getenv("R2_SECRET_ACCESS_KEY"),
		R2Bucket:           os.Getenv("R2_BUCKET"),
		R2PublicBaseURL:    os.Getenv("R2_PUBLIC_BASE_URL"),
	}

	switch provider {
	case "local":
		// no-op; defaults apply
	case "r2":
		missing := []string{}
		if cfg.R2AccountID == "" {
			missing = append(missing, "R2_ACCOUNT_ID")
		}
		if cfg.R2AccessKeyID == "" {
			missing = append(missing, "R2_ACCESS_KEY_ID")
		}
		if cfg.R2SecretAccessKey == "" {
			missing = append(missing, "R2_SECRET_ACCESS_KEY")
		}
		if cfg.R2Bucket == "" {
			missing = append(missing, "R2_BUCKET")
		}
		if cfg.R2PublicBaseURL == "" {
			missing = append(missing, "R2_PUBLIC_BASE_URL")
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf("STORAGE_PROVIDER=r2 requires: %s", strings.Join(missing, ", "))
		}
	default:
		return nil, fmt.Errorf("STORAGE_PROVIDER must be 'local' or 'r2', got %q", provider)
	}

	return cfg, nil
}

func getEnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
