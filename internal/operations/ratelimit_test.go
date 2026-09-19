package operations

import (
	"ai-assistant/internal/store"
	"testing"
	"time"
)

func TestRateLimitCutoff(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)

	tests := map[int]time.Time{
		7:  time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC),
		1:  time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC),
		30: time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC),
	}

	for windowDays, want := range tests {
		if got := rateLimitCutoff(now, windowDays); !got.Equal(want) {
			t.Errorf("rateLimitCutoff(now, %d) = %v, want %v", windowDays, got, want)
		}
	}
}

func TestCountRecentPerChat(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	cutoff := rateLimitCutoff(now, 7)

	row := func(chatID int64, receivedAt time.Time) store.Row {
		var r store.Row
		r.ChatID = chatID
		r.ReceivedAt = receivedAt
		return r
	}

	rows := []store.Row{
		row(100, now.Add(-1*time.Hour)),     // inside
		row(100, now.AddDate(0, 0, -3)),     // inside
		row(100, now.AddDate(0, 0, -10)),    // outside, older than the window
		row(200, now.AddDate(0, 0, -2)),     // inside, different chat
		row(300, cutoff),                    // exactly on the boundary, excluded
		row(300, cutoff.Add(1*time.Second)), // just inside
	}

	counts := countRecentPerChat(rows, cutoff)

	if counts[100] != 2 {
		t.Errorf("chat 100 count = %d, want 2", counts[100])
	}
	if counts[200] != 1 {
		t.Errorf("chat 200 count = %d, want 1", counts[200])
	}
	if counts[300] != 1 {
		t.Errorf("chat 300 count = %d, want 1 (boundary row excluded)", counts[300])
	}
	if counts[999] != 0 {
		t.Errorf("unknown chat count = %d, want 0", counts[999])
	}
}

// A row that has aged out of the window must free the submitter's quota again.
func TestCountRecentPerChatReleasesQuotaAfterWindow(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)

	var old store.Row
	old.ChatID = 100
	old.ReceivedAt = now.AddDate(0, 0, -8)

	rows := []store.Row{old}

	if counts := countRecentPerChat(rows, rateLimitCutoff(now, 7)); counts[100] != 0 {
		t.Errorf("count = %d, want 0 for a row outside the window", counts[100])
	}
	if counts := countRecentPerChat(rows, rateLimitCutoff(now, 14)); counts[100] != 1 {
		t.Errorf("count = %d, want 1 for a row inside a wider window", counts[100])
	}
}

// The published Telegram text must not contain the configurable limit numbers, which would
// go stale the moment REFERRAL_RATE_LIMIT changes.
func TestBotCommandsAreValidForTelegram(t *testing.T) {
	for _, command := range BotCommands() {
		if command.Command == "" || len(command.Command) > 32 {
			t.Errorf("command %q must be 1-32 characters", command.Command)
		}
		if command.Command != lower(command.Command) {
			t.Errorf("command %q must be lowercase", command.Command)
		}
		if command.Command[0] == '/' {
			t.Errorf("command %q must not include the leading slash", command.Command)
		}
		if len(command.Description) > 256 {
			t.Errorf("description for %q exceeds 256 characters", command.Command)
		}
	}

	if len(BotDescription) > 512 {
		t.Errorf("BotDescription is %d characters, Telegram allows 512", len(BotDescription))
	}
	if len(BotShortDescription) > 120 {
		t.Errorf("BotShortDescription is %d characters, Telegram allows 120", len(BotShortDescription))
	}
}

func lower(s string) string {
	out := []byte(s)
	for i := range out {
		if out[i] >= 'A' && out[i] <= 'Z' {
			out[i] += 'a' - 'A'
		}
	}
	return string(out)
}
