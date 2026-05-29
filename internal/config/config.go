package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	defaultSummaryHourUTC = 18
	defaultHTTPTimeoutSec = 30
)

// Config represents the application runtime configuration loaded from environment variables.
type Config struct {
	TelegramBotToken      string
	DatabaseURL           string
	AnthropicAPIKey       string
	AnthropicModel        string
	OpenAIAPIKey          string
	OpenAIWhisperModel    string
	SupabaseURL           string
	SupabaseServiceRole   string
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
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		AnthropicAPIKey:       os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicModel:        getEnv("ANTHROPIC_MODEL", "claude-3-5-haiku-latest"),
		OpenAIAPIKey:          os.Getenv("OPENAI_API_KEY"),
		OpenAIWhisperModel:    getEnv("OPENAI_WHISPER_MODEL", "whisper-1"),
		SupabaseURL:           os.Getenv("SUPABASE_URL"),
		SupabaseServiceRole:   os.Getenv("SUPABASE_SERVICE_ROLE_KEY"),
		SupabaseStorageBucket: getEnv("SUPABASE_STORAGE_BUCKET", "chatvault"),
		NotionVersion:         getEnv("NOTION_VERSION", "2022-06-28"),
		DailySummaryHourUTC:   hour,
		DailySummaryMinuteUTC: minute,
		HTTPTimeout:           time.Duration(getEnvInt("HTTP_TIMEOUT_SECONDS", defaultHTTPTimeoutSec)) * time.Second,
	}

	if cfg.TelegramBotToken == "" {
		return Config{}, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
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
