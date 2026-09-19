package main

import (
	"ai-assistant/configs"
	"ai-assistant/internal/clients"
	"ai-assistant/internal/operations"
	"context"
	"log"
	"time"
)

const runTimeout = 30 * time.Second

// botsetup publishes the command menu and descriptions to Telegram.
//
// This is a one-off: the settings persist on Telegram's servers, so running it from the cron
// job would spend three API calls every ten minutes re-sending text that never changes.
// Re-run it manually whenever the command wording is edited.
func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	config := configs.InitConfig()
	if config.BotToken == "" {
		log.Fatal("BOT_TOKEN is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	bot := clients.NewTeleClient(config.BotToken)

	if err := bot.SetMyCommands(ctx, operations.BotCommands()); err != nil {
		log.Fatalf("Failed to set commands: %v", err)
	}
	log.Println("Registered command menu")

	if err := bot.SetMyDescription(ctx, operations.BotDescription); err != nil {
		log.Fatalf("Failed to set description: %v", err)
	}
	log.Println("Registered description")

	if err := bot.SetMyShortDescription(ctx, operations.BotShortDescription); err != nil {
		log.Fatalf("Failed to set short description: %v", err)
	}
	log.Println("Registered short description")

	log.Println("Bot metadata updated. Open a fresh chat with the bot to see the description.")
}
