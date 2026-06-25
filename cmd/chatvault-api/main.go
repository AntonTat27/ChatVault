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
	log.Println("Starting dashboard API...")
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}
	if cfg.SessionSecret == "" {
		log.Fatalf("SESSION_SECRET is required to run the dashboard api")
	}
	log.Println("Config loaded successfully")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("Initializing Supabase repository...")
	repo := storage.NewRepository(cfg.SupabaseURL, cfg.SupabaseSecretKey, cfg.HTTPTimeout)
	log.Println("Supabase repository initialized")

	log.Println("Initializing Gemini client...")
	geminiClient := ai.NewGeminiClient(cfg.GeminiAPIKey, cfg.GeminiClassificationModel, cfg.GeminiSummaryModel, cfg.GeminiEmbeddingModel, cfg.HTTPTimeout)
	log.Println("Gemini client initialized")

	var notionCipher *crypto.Cipher
	if cfg.NotionEncryptionKey != "" {
		notionCipher, err = crypto.NewCipher(cfg.NotionEncryptionKey)
		if err != nil {
			log.Fatalf("notion encryption key invalid: %v", err)
		}
	}

	log.Println("Initializing services...")
	// transcriber/storage clients are unused by dashboard API routes; passing
	// nil keeps this binary from depending on Supabase Storage credentials it
	// has no use for. notionClient stays nil too -- the dashboard never posts
	// summary pages itself, only handles the OAuth handshake and DB picker.
	services := service.NewServices(ctx, repo, geminiClient, nil, nil, nil, notionCipher, cfg.GeminiEmbeddingModel, cfg.DailySummaryHourUTC, cfg.DailySummaryMinuteUTC)
	defer services.Close()
	log.Println("Services initialized")

	log.Println("Initializing Telegram bot client...")
	// Constructed without Start(): used only for synchronous Bot API calls
	// (getChatMember), never for long-polling updates.
	telegramBot, err := telegrambot.New(cfg.TelegramBotToken)
	if err != nil {
		log.Fatalf("telegram bot client init failed: %v", err)
	}
	log.Println("Telegram bot client initialized")

	notionOAuthCfg := notion.OAuthConfig{
		ClientID:     cfg.NotionOAuthClientID,
		ClientSecret: cfg.NotionOAuthClientSecret,
		RedirectURL:  cfg.NotionOAuthRedirectURL,
	}

	log.Println("Creating API handler and router...")
	handler := api.NewHandler(services, repo, telegramBot, cfg.TelegramBotToken, notionOAuthCfg, cfg.SessionSecret, cfg.DashboardBaseURL, cfg.HTTPTimeout)
	router := api.NewRouter(handler, telegramBot, repo, cfg.AllowedOrigins)
	server := api.NewServer(cfg.APIPort, router)
	log.Println("API server created, starting...")

	if err := server.Run(ctx); err != nil {
		log.Fatalf("dashboard api server failed: %v", err)
	}
}
