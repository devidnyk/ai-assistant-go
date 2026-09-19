package clients

import (
	"ai-assistant/internal/models"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const BotAPIBaseURL = "https://api.telegram.org/bot"

// defaultTimeout bounds every Bot API call. Without it a hung connection would stall the
// cron job until the GitHub Actions runner is killed.
const defaultTimeout = 30 * time.Second

type TeleClient struct {
	botToken   string
	httpClient *http.Client
}

func NewTeleClient(botToken string) *TeleClient {
	httpClient := http.Client{Timeout: defaultTimeout}
	return &TeleClient{botToken: botToken, httpClient: &httpClient}
}

func NewTeleClientWithHttpClient(botToken string, httpClient *http.Client) *TeleClient {
	return &TeleClient{botToken: botToken, httpClient: httpClient}
}

func (tc *TeleClient) GetBotInfo(ctx context.Context) (*models.User, error) {
	raw, err := tc.apiCall(ctx, "getMe", nil)
	if err != nil {
		return nil, err
	}

	var user models.User
	if err := json.Unmarshal(raw, &user); err != nil {
		return nil, fmt.Errorf("decode getMe result: %w", err)
	}

	return &user, nil
}

// GetUpdates fetches pending updates starting at offset. Passing an offset acknowledges every
// update below it, so callers must only advance the offset once the batch is safely persisted.
func (tc *TeleClient) GetUpdates(ctx context.Context, offset int64, limit int, timeout int) ([]models.Update, error) {
	payload := models.GetUpdatesRequest{
		Offset:         offset,
		Limit:          limit,
		Timeout:        timeout,
		AllowedUpdates: []string{"message"},
	}

	raw, err := tc.apiCall(ctx, "getUpdates", payload)
	if err != nil {
		return nil, err
	}

	var updates []models.Update
	if err := json.Unmarshal(raw, &updates); err != nil {
		return nil, fmt.Errorf("decode getUpdates result: %w", err)
	}

	return updates, nil
}

// SendMessage delivers plain text to a chat. Errors from Telegram are surfaced as
// *models.APIError so callers can distinguish permanent failures from transient ones.
func (tc *TeleClient) SendMessage(ctx context.Context, chatID int64, text string) error {
	payload := models.SendMessageRequest{
		ChatID:                chatID,
		Text:                  text,
		DisableWebPagePreview: true,
	}

	_, err := tc.apiCall(ctx, "sendMessage", payload)
	return err
}

// SetMyCommands publishes the command menu shown beside the message box and in "/"
// autocomplete. This is a one-off registration that persists on Telegram's servers.
func (tc *TeleClient) SetMyCommands(ctx context.Context, commands []models.BotCommand) error {
	_, err := tc.apiCall(ctx, "setMyCommands", models.SetMyCommandsRequest{Commands: commands})
	return err
}

// SetMyDescription sets the text shown on the empty chat screen, before a user sends /start.
func (tc *TeleClient) SetMyDescription(ctx context.Context, description string) error {
	_, err := tc.apiCall(ctx, "setMyDescription", models.SetMyDescriptionRequest{Description: description})
	return err
}

// SetMyShortDescription sets the text shown on the bot's profile page.
func (tc *TeleClient) SetMyShortDescription(ctx context.Context, shortDescription string) error {
	_, err := tc.apiCall(ctx, "setMyShortDescription", models.SetMyShortDescriptionRequest{
		ShortDescription: shortDescription,
	})
	return err
}

// apiCall performs a Bot API request and unwraps the standard response envelope.
func (tc *TeleClient) apiCall(ctx context.Context, method string, payload any) (json.RawMessage, error) {
	// The bot token is part of the URL, so the URL must never be logged.
	log.Printf("Calling Telegram API method: %s", method)

	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode %s request: %w", method, err)
		}
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, BotAPIBaseURL+tc.botToken+"/"+method, body)
	if err != nil {
		return nil, fmt.Errorf("create %s request: %w", method, err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	raw, err := SendHttpRequestResponseBase(tc.httpClient, req)
	if err != nil {
		return nil, fmt.Errorf("telegram %s transport error: %w", method, err)
	}

	var envelope models.APIResponse
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return nil, fmt.Errorf("telegram %s returned invalid JSON: %w", method, err)
	}

	if !envelope.OK {
		return nil, &models.APIError{
			Method:      method,
			ErrorCode:   envelope.ErrorCode,
			Description: envelope.Description,
		}
	}

	return envelope.Result, nil
}
