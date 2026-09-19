package operations

import (
	"ai-assistant/internal/clients"
	"ai-assistant/internal/models"
	"ai-assistant/internal/store"
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"
)

const (
	ReferCommand = "/refer"
	HelpCommand  = "/help"
	StartCommand = "/start"

	maxCandidateNameLen = 100
	maxJobIDLen         = 64
	maxResumeURLLen     = 500
)

// ErrNotReferCommand signals the message was not a referral submission at all.
var ErrNotReferCommand = errors.New("not a /refer command")

// BotDescription is published to Telegram and shown on the empty chat screen, before anyone
// sends /start. It is the only place the format is visible without typing anything first.
//
// It deliberately omits the rate limit numbers: this text is registered once on Telegram's
// servers, whereas the limit is runtime configuration, so embedding the numbers here would
// silently start lying the moment REFERRAL_RATE_LIMIT changes.
const BotDescription = "I collect job referral requests.\n\n" +
	"Send:\n" +
	"/refer <candidate name>, <job id>, <resume link>\n\n" +
	"Example:\n" +
	"/refer Jane Doe, JOB-12345, https://drive.google.com/file/d/abc123/view\n\n" +
	"You will get a message here once the referral has been submitted."

// BotShortDescription appears on the bot profile page and in shared link previews.
const BotShortDescription = "Send me a job referral request and I will log it for review."

// BotCommands is the menu Telegram shows next to the message box and in "/" autocomplete.
// Names are lowercase and carry no leading slash, as the Bot API requires.
func BotCommands() []models.BotCommand {
	return []models.BotCommand{
		{Command: "refer", Description: "Submit a referral: /refer Name, Job ID, Resume link"},
		{Command: "help", Description: "Show the referral request format"},
	}
}

// usageMessage is the runtime reply for /start, /help and malformed input. It is derived from
// BotDescription so the published text and the in-chat text cannot drift apart.
const usageMessage = BotDescription

// ParsedReferral holds the three validated fields extracted from a /refer message.
type ParsedReferral struct {
	CandidateName string
	JobID         string
	ResumeURL     string
}

// ReferralOperation runs the two passes of the cron job: draining the Telegram inbox and
// notifying submitters whose referrals have been marked Done in the spreadsheet.
type ReferralOperation struct {
	Bot                 *clients.TeleClient
	Store               *store.SheetStore
	RateLimit           int
	RateLimitWindowDays int
	PollLimit           int

	// Now is injectable so rate limiting and timestamps can be tested deterministically.
	Now func() time.Time
}

func (op *ReferralOperation) now() time.Time {
	if op.Now != nil {
		return op.Now()
	}
	return time.Now()
}

// CommandOf returns the lowercase bot command a message starts with, stripped of any
// @botname suffix. It returns an empty string when the message is not a command.
func CommandOf(text string) string {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "/") {
		return ""
	}

	command := trimmed
	if idx := strings.IndexAny(trimmed, " \t\n"); idx != -1 {
		command = trimmed[:idx]
	}

	// Group chats deliver commands as "/refer@MyBot".
	if idx := strings.Index(command, "@"); idx != -1 {
		command = command[:idx]
	}

	return strings.ToLower(command)
}

// ParseReferCommand extracts and validates the candidate name, job id and resume link.
//
// The arguments are split on the last two commas rather than the first two, so a candidate
// name containing a comma (for example "Doe, Jane") still parses correctly.
func ParseReferCommand(text string) (*ParsedReferral, error) {
	if CommandOf(text) != ReferCommand {
		return nil, ErrNotReferCommand
	}

	trimmed := strings.TrimSpace(text)
	args := ""
	if idx := strings.IndexAny(trimmed, " \t\n"); idx != -1 {
		args = strings.TrimSpace(trimmed[idx+1:])
	}

	if args == "" {
		return nil, errors.New("no details provided")
	}

	lastComma := strings.LastIndex(args, ",")
	if lastComma == -1 {
		return nil, errors.New("expected 3 comma-separated values: candidate name, job id, resume link")
	}

	head := args[:lastComma]
	resumeURL := strings.TrimSpace(args[lastComma+1:])

	secondComma := strings.LastIndex(head, ",")
	if secondComma == -1 {
		return nil, errors.New("expected 3 comma-separated values: candidate name, job id, resume link")
	}

	candidateName := strings.TrimSpace(head[:secondComma])
	jobID := strings.TrimSpace(head[secondComma+1:])

	if candidateName == "" {
		return nil, errors.New("candidate name is empty")
	}
	if jobID == "" {
		return nil, errors.New("job id is empty")
	}
	if resumeURL == "" {
		return nil, errors.New("resume link is empty")
	}

	if len(candidateName) > maxCandidateNameLen {
		return nil, fmt.Errorf("candidate name is too long (max %d characters)", maxCandidateNameLen)
	}
	if len(jobID) > maxJobIDLen {
		return nil, fmt.Errorf("job id is too long (max %d characters)", maxJobIDLen)
	}
	if len(resumeURL) > maxResumeURLLen {
		return nil, fmt.Errorf("resume link is too long (max %d characters)", maxResumeURLLen)
	}

	if err := validateResumeURL(resumeURL); err != nil {
		return nil, err
	}

	return &ParsedReferral{
		CandidateName: candidateName,
		JobID:         jobID,
		ResumeURL:     resumeURL,
	}, nil
}

// validateResumeURL rejects anything that is not an absolute http(s) link, which keeps
// javascript:, data: and file: payloads out of the spreadsheet.
func validateResumeURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return errors.New("resume link is not a valid URL")
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("resume link must start with http:// or https://")
	}
	if parsed.Host == "" {
		return errors.New("resume link is missing a domain")
	}

	return nil
}

// ProcessInbox pulls new Telegram messages, records valid referrals and replies to senders.
//
// The Telegram offset is advanced only after the rows are durably written, and updates whose
// id already exists in the sheet are skipped, so a crash at any point cannot lose or duplicate
// a submission.
func (op *ReferralOperation) ProcessInbox(ctx context.Context) error {
	rows, err := op.Store.LoadAll(ctx)
	if err != nil {
		return err
	}

	offset, err := op.Store.GetOffset(ctx)
	if err != nil {
		return err
	}

	updates, err := op.Bot.GetUpdates(ctx, offset, op.PollLimit, 0)
	if err != nil {
		return err
	}

	if len(updates) == 0 {
		log.Println("No new Telegram updates")
		return nil
	}

	log.Printf("Fetched %d Telegram update(s) from offset %d", len(updates), offset)

	seen := make(map[int64]bool, len(rows))
	submitted := make(map[string]bool, len(rows))
	for _, row := range rows {
		seen[row.UpdateID] = true
		submitted[DedupKey(row.ChatID, row.CandidateName, row.JobID)] = true
	}

	cutoff := rateLimitCutoff(op.now(), op.RateLimitWindowDays)
	recentPerChat := countRecentPerChat(rows, cutoff)

	type outgoing struct {
		chatID int64
		text   string
	}

	newReferrals := make([]models.Referral, 0, len(updates))
	replies := make([]outgoing, 0, len(updates))
	newOffset := offset

	for _, update := range updates {
		if update.UpdateID >= newOffset {
			newOffset = update.UpdateID + 1
		}

		message := update.Message
		if message == nil || message.Chat == nil || strings.TrimSpace(message.Text) == "" {
			continue
		}
		if seen[update.UpdateID] {
			log.Printf("Skipping update %d, already recorded", update.UpdateID)
			continue
		}

		chatID := message.Chat.ID
		command := CommandOf(message.Text)

		if command != ReferCommand {
			if command == StartCommand || command == HelpCommand {
				replies = append(replies, outgoing{chatID, usageMessage})
			} else {
				replies = append(replies, outgoing{chatID, "I only handle referral requests.\n\n" + usageMessage})
			}
			continue
		}

		parsed, parseErr := ParseReferCommand(message.Text)
		if parseErr != nil {
			replies = append(replies, outgoing{chatID, "Could not read that request: " + parseErr.Error() + "\n\n" + usageMessage})
			continue
		}

		// Distinct Telegram messages carrying the same referral are still duplicates. The
		// update_id guard cannot catch these, since each message is genuinely new.
		// Checked before the rate limit so a duplicate does not consume the sender's quota.
		dedupKey := DedupKey(chatID, parsed.CandidateName, parsed.JobID)
		if submitted[dedupKey] {
			log.Printf("Rejecting duplicate referral from chat %d for job %s", chatID, parsed.JobID)
			replies = append(replies, outgoing{chatID, fmt.Sprintf(
				"You have already submitted a referral for %s (job %s). It is still on the list, no need to send it again.",
				parsed.CandidateName, parsed.JobID)})
			continue
		}

		if recentPerChat[chatID] >= op.RateLimit {
			replies = append(replies, outgoing{chatID, fmt.Sprintf(
				"You have reached the limit of %d referral request(s) every %d day(s). Please try again later.",
				op.RateLimit, op.RateLimitWindowDays)})
			continue
		}
		recentPerChat[chatID]++
		submitted[dedupKey] = true

		referral := models.Referral{
			UpdateID:      update.UpdateID,
			ReceivedAt:    op.now(),
			ChatID:        chatID,
			CandidateName: parsed.CandidateName,
			JobID:         parsed.JobID,
			ResumeURL:     parsed.ResumeURL,
			Status:        models.StatusPending,
		}
		if message.From != nil {
			referral.UserID = message.From.ID
			referral.Username = message.From.Username
			referral.FirstName = message.From.FirstName
		}

		newReferrals = append(newReferrals, referral)
		replies = append(replies, outgoing{chatID, fmt.Sprintf(
			"Received. Your referral request has been logged:\n\nCandidate: %s\nJob ID: %s\nResume: %s\n\nStatus: %s. You will get a message here once it has been submitted.",
			parsed.CandidateName, parsed.JobID, parsed.ResumeURL, models.StatusPending)})
	}

	// Persist before acknowledging anything, so a failure here replays the whole batch.
	if err := op.Store.AppendReferrals(ctx, newReferrals); err != nil {
		return err
	}
	log.Printf("Recorded %d new referral(s)", len(newReferrals))

	// Replies are best effort: a blocked user must not prevent the offset from advancing,
	// otherwise the job would reprocess the same batch forever.
	for _, reply := range replies {
		if err := op.Bot.SendMessage(ctx, reply.chatID, reply.text); err != nil {
			log.Printf("Failed to reply to chat %d: %v", reply.chatID, err)
		}
	}

	if err := op.Store.SetOffset(ctx, newOffset); err != nil {
		return err
	}
	log.Printf("Advanced Telegram offset to %d", newOffset)

	return nil
}

// NotifyCompleted messages every submitter whose row is marked Done but not yet notified.
//
// Messages are sent before the notified_at stamp is written, making delivery at-least-once: a
// crash in between can repeat a notification, which is harmless, whereas stamping first could
// silently drop one.
func (op *ReferralOperation) NotifyCompleted(ctx context.Context) error {
	rows, err := op.Store.LoadAll(ctx)
	if err != nil {
		return err
	}

	notified := 0
	transientFailures := 0

	for _, row := range rows {
		if !row.IsDone() || row.IsNotified() {
			continue
		}

		if row.ChatID == 0 {
			log.Printf("Row %d is Done but has no chat id, skipping", row.Index)
			continue
		}

		sendErr := op.Bot.SendMessage(ctx, row.ChatID, completionMessage(row.Referral))

		var apiErr *models.APIError
		switch {
		case sendErr == nil:
			if err := op.Store.MarkNotified(ctx, row.Index, op.now(), ""); err != nil {
				// The message went out but the stamp did not land, so the next run will
				// repeat it. Surface this rather than hiding it.
				log.Printf("Notified chat %d but failed to stamp row %d: %v", row.ChatID, row.Index, err)
				transientFailures++
				continue
			}
			notified++

		case errors.As(sendErr, &apiErr) && apiErr.IsPermanent():
			// The chat is gone or the bot is blocked. Stamp it so this row stops retrying.
			log.Printf("Permanent delivery failure for row %d: %v", row.Index, sendErr)
			if err := op.Store.MarkNotified(ctx, row.Index, op.now(), apiErr.Error()); err != nil {
				log.Printf("Failed to record permanent failure on row %d: %v", row.Index, err)
				transientFailures++
			}

		default:
			// Transient: leave notified_at empty so the next run retries.
			log.Printf("Temporary delivery failure for row %d, will retry: %v", row.Index, sendErr)
			transientFailures++
		}
	}

	log.Printf("Sent %d completion notification(s)", notified)

	if transientFailures > 0 {
		return fmt.Errorf("%d completion notification(s) failed and will be retried", transientFailures)
	}

	return nil
}

func completionMessage(referral models.Referral) string {
	return fmt.Sprintf(
		"Good news: the referral for %s (job %s) has been submitted.",
		referral.CandidateName, referral.JobID)
}

// rateLimitCutoff returns the start of the rolling rate limit window.
//
// AddDate is used instead of subtracting windowDays*24h so the window stays aligned to
// calendar days across daylight saving transitions.
func rateLimitCutoff(now time.Time, windowDays int) time.Time {
	return now.AddDate(0, 0, -windowDays)
}

// countRecentPerChat tallies submissions per chat within the rate limit window.
func countRecentPerChat(rows []store.Row, cutoff time.Time) map[int64]int {
	counts := make(map[int64]int)

	for _, row := range rows {
		if row.ReceivedAt.After(cutoff) {
			counts[row.ChatID]++
		}
	}

	return counts
}

// DedupKey identifies a referral by sender, candidate and job, so the same request sent twice
// as two separate Telegram messages is recognised as a duplicate.
//
// The resume link is deliberately excluded: re-sending the same candidate for the same job
// with an updated CV link is still the same referral, not a new one.
//
// Fields are length-prefixed rather than simply joined by a separator, because a candidate
// name containing the separator could otherwise produce the same key as a different
// candidate/job pair.
func DedupKey(chatID int64, candidateName string, jobID string) string {
	name := normalizeKeyPart(candidateName)
	job := normalizeKeyPart(jobID)

	return fmt.Sprintf("%d|%d:%s|%d:%s", chatID, len(name), name, len(job), job)
}

// normalizeKeyPart lowercases and collapses whitespace so "Jane  Doe" and "jane doe" match.
func normalizeKeyPart(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}
