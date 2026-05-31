package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	defaultSummaryHourUTC = 18
	defaultHTTPTimeoutSec = 120
)

// Config represents the application runtime configuration loaded from environment variables.
type Config struct {
	TelegramBotToken      string
	GeminiAPIKey          string
	GeminiModel           string
	GeminiTranscribeModel string
	SupabaseURL           string
	SupabaseSecretKey     string
	SupabaseStorageBucket string
	NotionVersion         string
	DailySummaryHourUTC   int
	DailySummaryMinuteUTC int
	HTTPTimeout           time.Duration
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
		TelegramBotToken:      os.Getenv("TELEGRAM_BOT_TOKEN"),
		GeminiAPIKey:          os.Getenv("GEMINI_API_KEY"),
		GeminiModel:           getEnv("GEMINI_MODEL", "gemini-2.5-flash"),
		GeminiTranscribeModel: getEnv("GEMINI_TRANSCRIBE_MODEL", "gemini-2.5-flash"),
		SupabaseURL:           os.Getenv("SUPABASE_URL"),
		SupabaseSecretKey:     os.Getenv("SUPABASE_SECRET_KEY"),
		SupabaseStorageBucket: getEnv("SUPABASE_STORAGE_BUCKET", "chatvault"),
		NotionVersion:         getEnv("NOTION_VERSION", "2022-06-28"),
		DailySummaryHourUTC:   hour,
		DailySummaryMinuteUTC: minute,
		HTTPTimeout:           time.Duration(getEnvInt("HTTP_TIMEOUT_SECONDS", defaultHTTPTimeoutSec)) * time.Second,
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
