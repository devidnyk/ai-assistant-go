package configs

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const (
	// DefaultRateLimit caps how many referrals a single Telegram chat may submit per window.
	DefaultRateLimit = 1
	// DefaultRateLimitWindowDays is the length of the rolling rate limit window.
	DefaultRateLimitWindowDays = 7
	// DefaultPollLimit is how many Telegram updates to pull in one getUpdates call (max 100).
	DefaultPollLimit = 100
)

type Config struct {
	// Telegram
	BotToken string

	// Google Sheets referral store
	SpreadsheetID      string
	GoogleSAJSONBase64 string

	// Referral inbox behaviour
	RateLimit           int
	RateLimitWindowDays int
	PollLimit           int

	// Optional AI/RAG configuration. Only required by the CLI assistant, not the referral inbox.
	GeminiApiKey  string
	QdrantApiKey  string
	SysPromptPath string
}

// InitConfig reads configuration from the environment. A .env file is loaded when present
// (local development) but its absence is not an error, since CI supplies real environment
// variables instead. Validation is deferred to the Validate* helpers so that each entrypoint
// only demands the keys it actually needs.
func InitConfig() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file loaded, falling back to process environment:", err)
	}

	return &Config{
		BotToken: os.Getenv("BOT_TOKEN"),

		SpreadsheetID:      normalizeSpreadsheetID(os.Getenv("SPREADSHEET_ID")),
		GoogleSAJSONBase64: os.Getenv("GOOGLE_SA_JSON_B64"),

		RateLimit:           envInt("REFERRAL_RATE_LIMIT", DefaultRateLimit),
		RateLimitWindowDays: envInt("REFERRAL_RATE_LIMIT_WINDOW_DAYS", DefaultRateLimitWindowDays),
		PollLimit:           envInt("TELEGRAM_POLL_LIMIT", DefaultPollLimit),

		GeminiApiKey:  os.Getenv("GEMINI_API_KEY"),
		QdrantApiKey:  os.Getenv("QDRANT_API_KEY"),
		SysPromptPath: os.Getenv("SYS_PROMPT_PATH"),
	}
}

// ValidateForInbox checks the keys the referral inbox cron job requires.
func (c *Config) ValidateForInbox() error {
	missing := make([]string, 0, 3)

	if c.BotToken == "" {
		missing = append(missing, "BOT_TOKEN")
	}
	if c.SpreadsheetID == "" {
		missing = append(missing, "SPREADSHEET_ID")
	}
	if c.GoogleSAJSONBase64 == "" {
		missing = append(missing, "GOOGLE_SA_JSON_B64")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %v", missing)
	}

	if c.RateLimit <= 0 {
		return fmt.Errorf("REFERRAL_RATE_LIMIT must be positive, got %d", c.RateLimit)
	}
	if c.RateLimitWindowDays <= 0 {
		return fmt.Errorf("REFERRAL_RATE_LIMIT_WINDOW_DAYS must be positive, got %d", c.RateLimitWindowDays)
	}
	if c.PollLimit <= 0 || c.PollLimit > 100 {
		return fmt.Errorf("TELEGRAM_POLL_LIMIT must be between 1 and 100, got %d", c.PollLimit)
	}

	return nil
}

// ValidateForAssistant checks the keys the RAG assistant CLI requires.
func (c *Config) ValidateForAssistant() error {
	missing := make([]string, 0, 3)

	if c.GeminiApiKey == "" {
		missing = append(missing, "GEMINI_API_KEY")
	}
	if c.QdrantApiKey == "" {
		missing = append(missing, "QDRANT_API_KEY")
	}
	if c.SysPromptPath == "" {
		missing = append(missing, "SYS_PROMPT_PATH")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %v", missing)
	}

	return nil
}

// normalizeSpreadsheetID accepts either a bare spreadsheet ID or a full Google Sheets URL.
// Pasting the URL is the natural mistake and otherwise surfaces as an opaque 404 from the API.
func normalizeSpreadsheetID(value string) string {
	trimmed := strings.TrimSpace(value)

	const marker = "/spreadsheets/d/"
	idx := strings.Index(trimmed, marker)
	if idx == -1 {
		return trimmed
	}

	id := trimmed[idx+len(marker):]
	if end := strings.IndexAny(id, "/?#"); end != -1 {
		id = id[:end]
	}

	return id
}

func envInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		log.Printf("Invalid integer for %s (%q), using default %d", key, raw, fallback)
		return fallback
	}

	return value
}
