package configs

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken     string
	GeminiApiKey string
	QdrantApiKey string
}

func InitConfig() *Config {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	config := Config{}

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Panic("BOT_TOKEN is not set in the environment")
	}

	config.BotToken = botToken

	geminiApiKey := os.Getenv("GEMINI_API_KEY")
	if geminiApiKey == "" {
		log.Panic("GEMINI_API_KEY is not set in the environment")
	}

	config.GeminiApiKey = geminiApiKey

	qdrantApiKey := os.Getenv("QDRANT_API_KEY")
	if qdrantApiKey == "" {
		log.Panic("QDRANT_API_KEY is not set in the environment")
	}

	config.QdrantApiKey = qdrantApiKey

	return &config
}
