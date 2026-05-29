package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"

	telegrambot "github.com/go-telegram/bot"
	_ "github.com/lib/pq"

	"chatvault/internal/ai"
	bothandler "chatvault/internal/bot"
	"chatvault/internal/config"
	"chatvault/internal/notion"
	"chatvault/internal/service"
	"chatvault/internal/storage"
	"chatvault/internal/supabase"
)

// main initializes dependencies and starts the Telegram bot.
func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database open failed: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("database ping failed: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	repo := storage.NewRepository(db)
	anthropicClient := ai.NewAnthropicClient(cfg.AnthropicAPIKey, cfg.AnthropicModel, cfg.HTTPTimeout)
	whisperClient := ai.NewWhisperClient(cfg.OpenAIAPIKey, cfg.OpenAIWhisperModel, cfg.HTTPTimeout)
	storageClient := supabase.NewStorageClient(cfg.SupabaseURL, cfg.SupabaseServiceRole, cfg.SupabaseStorageBucket, cfg.HTTPTimeout)
	notionClient := notion.NewClient(cfg.HTTPTimeout, cfg.NotionVersion)

	services := service.NewServices(ctx, repo, anthropicClient, whisperClient, storageClient, notionClient, cfg.DailySummaryHourUTC, cfg.DailySummaryMinuteUTC)
	defer services.Close()

	handler := bothandler.NewHandler(services, cfg.TelegramBotToken)
	telegramBot, err := telegrambot.New(cfg.TelegramBotToken, telegrambot.WithDefaultHandler(handler.DefaultHandler))
	if err != nil {
		log.Fatalf("telegram bot init failed: %v", err)
	}
	handler.RegisterHandlers(telegramBot)

	go services.RunDailySummaryScheduler(ctx, handler.SendText)
	telegramBot.Start(ctx)
}
