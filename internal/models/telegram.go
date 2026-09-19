package models

import (
	"encoding/json"
	"fmt"
)

// APIResponse is the envelope every Telegram Bot API method returns.
type APIResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	ErrorCode   int             `json:"error_code,omitempty"`
	Description string          `json:"description,omitempty"`
}

// APIError is returned when Telegram responds with ok:false. The error code is preserved
// so callers can tell a permanently undeliverable chat apart from a transient outage.
type APIError struct {
	Method      string
	ErrorCode   int
	Description string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("telegram %s failed (%d): %s", e.Method, e.ErrorCode, e.Description)
}

// IsPermanent reports whether retrying this call could ever succeed. A 403 means the user
// blocked the bot and a 400 usually means the chat no longer exists; neither will recover,
// so callers must stop retrying instead of looping forever.
func (e *APIError) IsPermanent() bool {
	return e.ErrorCode == 400 || e.ErrorCode == 403
}

type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message,omitempty"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	From      *User  `json:"from,omitempty"`
	Chat      *Chat  `json:"chat"`
	Date      int64  `json:"date"`
	Text      string `json:"text,omitempty"`
}

type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

type Chat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

type GetUpdatesRequest struct {
	Offset         int64    `json:"offset,omitempty"`
	Limit          int      `json:"limit,omitempty"`
	Timeout        int      `json:"timeout,omitempty"`
	AllowedUpdates []string `json:"allowed_updates,omitempty"`
}

type SendMessageRequest struct {
	ChatID                int64  `json:"chat_id"`
	Text                  string `json:"text"`
	ParseMode             string `json:"parse_mode,omitempty"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview,omitempty"`
}

// BotCommand is one entry in the command menu Telegram shows beside the message box.
// Command must be lowercase, 1-32 characters, and given without the leading slash.
type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

type SetMyCommandsRequest struct {
	Commands []BotCommand `json:"commands"`
}

// SetMyDescriptionRequest sets the text shown on the empty chat screen, before /start.
type SetMyDescriptionRequest struct {
	Description string `json:"description"`
}

// SetMyShortDescriptionRequest sets the text shown on the bot's profile page.
type SetMyShortDescriptionRequest struct {
	ShortDescription string `json:"short_description"`
}
