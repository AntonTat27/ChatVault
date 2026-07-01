package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultSummaryHourUTC = 18
	defaultHTTPTimeoutSec = 120
)

// Config represents the application runtime configuration loaded from environment variables.
type Config struct {
	TelegramBotToken string
	GeminiAPIKey     string
	GeminiModel      string
	// Optional: separate models for classification and summarization.
	GeminiClassificationModel string
	GeminiSummaryModel        string
	GeminiTranscribeModel     string
	GeminiEmbeddingModel      string
	SupabaseURL               string
	SupabaseSecretKey         string
	SupabaseStorageBucket     string
	NotionVersion             string
	DailySummaryHourUTC       int
	DailySummaryMinuteUTC     int
	HTTPTimeout time.Duration
	// Dashboard (cmd/chatvault-api) configuration. Required only to run that binary.
	SessionSecret    string
	APIPort          string
	AllowedOrigins   []string
	DashboardBaseURL string
	// Notion OAuth (Phase 4). Required only once OAuth onboarding is enabled.
	NotionOAuthClientID     string
	NotionOAuthClientSecret string
	NotionOAuthRedirectURL  string
	NotionEncryptionKey     string
	// DevAuthBypass disables Telegram login signature and timestamp verification.
	// Must never be set in production.
	DevAuthBypass bool
	// Environment is "production" unless APP_ENV is explicitly set otherwise.
	// Defaulting to "production" (rather than defaulting to "development")
	// means a deployment that simply forgets to set APP_ENV is safe by
	// construction: DevAuthBypass refuses to run unless this is explicitly
	// "development", which requires deliberately setting APP_ENV locally.
	Environment string
}

// Load builds Config from environment variables and applies defaults.
func Load() (Config, error) {
	hour := getEnvInt("DAILY_SUMMARY_HOUR_UTC", defaultSummaryHourUTC)
	minute := getEnvInt("DAILY_SUMMARY_MINUTE_UTC", 0)
	if hour < 0 || hour > 23 {
		return Config{}, fmt.Errorf("DAILY_SUMMARY_HOUR_UTC must be in range [0,23]")
	}
	if minute < 0 || minute > 59 {
		return Config{}, fmt.Errorf("DAILY_SUMMARY_MINUTE_UTC must be in range [0,59]")
	}

	cfg := Config{
		TelegramBotToken:          os.Getenv("TELEGRAM_BOT_TOKEN"),
		GeminiAPIKey:              os.Getenv("GEMINI_API_KEY"),
		GeminiModel:               getEnv("GEMINI_MODEL", "gemma-4-26b-a4b-it"),
		GeminiClassificationModel: getEnv("GEMINI_CLASSIFICATION_MODEL", getEnv("GEMINI_MODEL", "gemma-4-26b-a4b-it")),
		GeminiSummaryModel:        getEnv("GEMINI_SUMMARY_MODEL", "gemini-2.0-flash"),
		GeminiTranscribeModel:     getEnv("GEMINI_TRANSCRIBE_MODEL", "gemini-2.5-flash"),
		GeminiEmbeddingModel:      getEnv("GEMINI_EMBEDDING_MODEL", "text-embedding-004"),
		SupabaseURL:               os.Getenv("SUPABASE_URL"),
		SupabaseSecretKey:         os.Getenv("SUPABASE_SECRET_KEY"),
		SupabaseStorageBucket:     getEnv("SUPABASE_STORAGE_BUCKET", "chatvault"),
		NotionVersion:             getEnv("NOTION_VERSION", "2022-06-28"),
		DailySummaryHourUTC:       hour,
		DailySummaryMinuteUTC:     minute,
		HTTPTimeout:               time.Duration(getEnvInt("HTTP_TIMEOUT_SECONDS", defaultHTTPTimeoutSec)) * time.Second,
		SessionSecret:             os.Getenv("SESSION_SECRET"),
		APIPort:                   getEnv("API_PORT", ":"+getEnv("PORT", "8081")),
		AllowedOrigins:            splitAndTrim(os.Getenv("ALLOWED_ORIGINS")),
		DashboardBaseURL:          os.Getenv("DASHBOARD_BASE_URL"),
		NotionOAuthClientID:       os.Getenv("NOTION_OAUTH_CLIENT_ID"),
		NotionOAuthClientSecret:   os.Getenv("NOTION_OAUTH_CLIENT_SECRET"),
		NotionOAuthRedirectURL:    os.Getenv("NOTION_OAUTH_REDIRECT_URL"),
		NotionEncryptionKey:       os.Getenv("NOTION_ENCRYPTION_KEY"),
		DevAuthBypass:             os.Getenv("DEV_AUTH_BYPASS") == "true",
		Environment:               getEnv("APP_ENV", "production"),
	}

	if cfg.TelegramBotToken == "" {
		return Config{}, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	if cfg.SupabaseURL == "" {
		return Config{}, fmt.Errorf("SUPABASE_URL is required")
	}
	if cfg.SupabaseSecretKey == "" {
		return Config{}, fmt.Errorf("SUPABASE_SECRET_KEY is required")
	}

	return cfg, nil
}

// getEnv returns an environment variable value or a fallback if unset.
func getEnv(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

// splitAndTrim splits a comma-separated env var into a trimmed, non-empty slice.
func splitAndTrim(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// getEnvInt returns an integer environment variable value or a fallback if unset or invalid.
func getEnvInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
