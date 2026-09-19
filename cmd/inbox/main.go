package main

import (
	"ai-assistant/configs"
	"ai-assistant/internal/clients"
	"ai-assistant/internal/operations"
	"ai-assistant/internal/store"
	"context"
	"log"
	"time"
)

// runTimeout bounds the whole job so a stuck API call cannot hold a CI runner open.
const runTimeout = 5 * time.Minute

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)
	log.Println("Starting referral inbox run")

	config := configs.InitConfig()
	if err := config.ValidateForInbox(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	sheetStore, err := store.NewSheetStore(ctx, config.GoogleSAJSONBase64, config.SpreadsheetID)
	if err != nil {
		log.Fatalf("Failed to connect to Google Sheets: %v", err)
	}

	if err := sheetStore.EnsureSchema(ctx); err != nil {
		log.Fatalf("Failed to prepare spreadsheet: %v", err)
	}

	referralOp := &operations.ReferralOperation{
		Bot:                 clients.NewTeleClient(config.BotToken),
		Store:               sheetStore,
		RateLimit:           config.RateLimit,
		RateLimitWindowDays: config.RateLimitWindowDays,
		PollLimit:           config.PollLimit,
	}

	// Both passes always run. An inbound failure must not stop people who are already
	// waiting on a completion notification from hearing back.
	inboxErr := referralOp.ProcessInbox(ctx)
	if inboxErr != nil {
		log.Printf("Inbox pass failed: %v", inboxErr)
	}

	notifyErr := referralOp.NotifyCompleted(ctx)
	if notifyErr != nil {
		log.Printf("Notification pass failed: %v", notifyErr)
	}

	if inboxErr != nil || notifyErr != nil {
		// Exit non-zero so the workflow goes red and GitHub emails about it. Silent failure
		// is dangerous here: Telegram discards undelivered updates after 24 hours.
		log.Fatal("Referral inbox run finished with errors")
	}

	log.Println("Referral inbox run completed")
}
