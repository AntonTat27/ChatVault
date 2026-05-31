package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	telegrambot "github.com/go-telegram/bot"

	"chatvault/internal/ai"
	bothandler "chatvault/internal/bot"
	"chatvault/internal/config"
	"chatvault/internal/notion"
	"chatvault/internal/service"
	"chatvault/internal/storage"
	"chatvault/internal/supabase"

	"github.com/joho/godotenv"
)

// main initializes dependencies and starts the Telegram bot.
func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	repo := storage.NewRepository(cfg.SupabaseURL, cfg.SupabaseSecretKey, cfg.HTTPTimeout)
	geminiClient := ai.NewGeminiClient(cfg.GeminiAPIKey, cfg.GeminiModel, cfg.HTTPTimeout)
	transcriberClient := ai.NewGeminiTranscribeClient(cfg.GeminiAPIKey, cfg.GeminiTranscribeModel, cfg.HTTPTimeout)
	storageClient := supabase.NewStorageClient(cfg.SupabaseURL, cfg.SupabaseSecretKey, cfg.SupabaseStorageBucket, cfg.HTTPTimeout)
	notionClient := notion.NewClient(cfg.HTTPTimeout, cfg.NotionVersion)

	services := service.NewServices(ctx, repo, geminiClient, transcriberClient, storageClient, notionClient, cfg.DailySummaryHourUTC, cfg.DailySummaryMinuteUTC)
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
