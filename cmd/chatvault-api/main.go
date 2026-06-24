package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	telegrambot "github.com/go-telegram/bot"

	"chatvault/internal/ai"
	"chatvault/internal/api"
	"chatvault/internal/config"
	"chatvault/internal/crypto"
	"chatvault/internal/db"
	"chatvault/internal/notion"
	"chatvault/internal/service"
	"chatvault/internal/storage"

	"github.com/joho/godotenv"
)

// main initializes dependencies and starts the dashboard's HTTP API. This is
// a separate binary from cmd/chatvault (the Telegram bot's long-polling
// process) so the dashboard can be deployed and scaled independently.
func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}
	if cfg.DatabaseURL == "" {
		log.Fatalf("DATABASE_URL is required to run the dashboard api")
	}
	if cfg.SessionSecret == "" {
		log.Fatalf("SESSION_SECRET is required to run the dashboard api")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbPool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database pool init failed: %v", err)
	}
	defer dbPool.Close()

	repo := storage.NewRepository(cfg.SupabaseURL, cfg.SupabaseSecretKey, cfg.HTTPTimeout)
	geminiClient := ai.NewGeminiClient(cfg.GeminiAPIKey, cfg.GeminiClassificationModel, cfg.GeminiSummaryModel, cfg.GeminiEmbeddingModel, cfg.HTTPTimeout)

	var notionCipher *crypto.Cipher
	if cfg.NotionEncryptionKey != "" {
		notionCipher, err = crypto.NewCipher(cfg.NotionEncryptionKey)
		if err != nil {
			log.Fatalf("notion encryption key invalid: %v", err)
		}
	}

	// transcriber/storage clients are unused by dashboard API routes; passing
	// nil keeps this binary from depending on Supabase Storage credentials it
	// has no use for. notionClient stays nil too -- the dashboard never posts
	// summary pages itself, only handles the OAuth handshake and DB picker.
	services := service.NewServices(ctx, repo, geminiClient, nil, nil, nil, notionCipher, dbPool, cfg.GeminiEmbeddingModel, cfg.DailySummaryHourUTC, cfg.DailySummaryMinuteUTC)
	defer services.Close()

	// Constructed without Start(): used only for synchronous Bot API calls
	// (getChatMember), never for long-polling updates.
	telegramBot, err := telegrambot.New(cfg.TelegramBotToken)
	if err != nil {
		log.Fatalf("telegram bot client init failed: %v", err)
	}

	notionOAuthCfg := notion.OAuthConfig{
		ClientID:     cfg.NotionOAuthClientID,
		ClientSecret: cfg.NotionOAuthClientSecret,
		RedirectURL:  cfg.NotionOAuthRedirectURL,
	}

	handler := api.NewHandler(services, dbPool, telegramBot, cfg.TelegramBotToken, notionOAuthCfg, cfg.SessionSecret, cfg.DashboardBaseURL, cfg.HTTPTimeout)
	router := api.NewRouter(handler, telegramBot, dbPool, cfg.AllowedOrigins)
	server := api.NewServer(cfg.APIPort, router)

	if err := server.Run(ctx); err != nil {
		log.Fatalf("dashboard api server failed: %v", err)
	}
}
