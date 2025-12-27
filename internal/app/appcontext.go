package app

import "ai-assistant/internal/clients"

type AppContext struct {
	GeminiClient *clients.GenaiClient
	QdrantClient *clients.QdrantClient
	BotClient    *clients.TeleClient
}

func NewAppContext() *AppContext {
	return &AppContext{}
}

func (ac *AppContext) InitClients(geminiClient *clients.GenaiClient, qdrantClient *clients.QdrantClient, botClient *clients.TeleClient) {
	ac.GeminiClient = geminiClient
	ac.QdrantClient = qdrantClient
	ac.BotClient = botClient
}
