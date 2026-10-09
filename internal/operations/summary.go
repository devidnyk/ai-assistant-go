package operations

import (
	"ai-assistant/internal/store"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
)

// RunSummary collects what one cron run did, plus a snapshot of the sheet taken after both
// passes. It feeds the log line, the GitHub step summary and the Telegram digest, so all three
// always report the same numbers.
type RunSummary struct {
	// Inbound pass.
	UpdatesFetched int
	NewReferrals   int
	Duplicates     int
	RateLimited    int
	Malformed      int

	// Outbound pass. The job never changes a row's status itself, since Pending -> Done is a
	// manual edit in the sheet. What it can observe is rows it newly notified as Done.
	Notified                int
	NotifyPermanentFailures int
	NotifyRetries           int

	// Sheet snapshot, taken after both passes. SnapshotOK is false if the sheet could not be
	// read, so the digest says "unknown" rather than reporting a misleading zero.
	SnapshotOK           bool
	Pending              int
	Done                 int
	AwaitingNotification int
}

// tallyRows counts rows by lifecycle stage. Anything that is not Done counts as pending, so a
// blank or mistyped status still shows up as outstanding work rather than vanishing.
func tallyRows(rows []store.Row) (pending int, done int, awaitingNotification int) {
	for _, row := range rows {
		if !row.IsDone() {
			pending++
			continue
		}

		done++
		if !row.IsNotified() {
			awaitingNotification++
		}
	}

	return pending, done, awaitingNotification
}

// Snapshot reads the sheet once and records the pending/done totals. It must run after both
// passes so the counts include rows added and notified during this run.
func (op *ReferralOperation) Snapshot(ctx context.Context) error {
	rows, err := op.Store.LoadAll(ctx)
	if err != nil {
		return err
	}

	op.Summary.Pending, op.Summary.Done, op.Summary.AwaitingNotification = tallyRows(rows)
	op.Summary.SnapshotOK = true

	return nil
}

func (s RunSummary) rejected() int {
	return s.Duplicates + s.RateLimited + s.Malformed
}

func (s RunSummary) deliveryProblems() int {
	return s.NotifyPermanentFailures + s.NotifyRetries
}

// HasActivity reports whether anything happened this run worth telling the owner about.
func (s RunSummary) HasActivity() bool {
	return s.NewReferrals > 0 || s.Notified > 0 || s.rejected() > 0 || s.deliveryProblems() > 0
}

// LogLine is a single greppable line for the workflow log.
func (s RunSummary) LogLine() string {
	sheet := "sheet: unavailable"
	if s.SnapshotOK {
		sheet = fmt.Sprintf("sheet: pending=%d done=%d awaiting_notification=%d",
			s.Pending, s.Done, s.AwaitingNotification)
	}

	return fmt.Sprintf(
		"Run summary: fetched=%d recorded=%d duplicates=%d rate_limited=%d malformed=%d "+
			"notified_done=%d notify_failed_permanent=%d notify_will_retry=%d | %s",
		s.UpdatesFetched, s.NewReferrals, s.Duplicates, s.RateLimited, s.Malformed,
		s.Notified, s.NotifyPermanentFailures, s.NotifyRetries, sheet)
}

// Markdown renders the summary for the GitHub Actions run page.
func (s RunSummary) Markdown() string {
	var b strings.Builder

	b.WriteString("### Referral inbox run\n\n")
	b.WriteString("| Metric | Count |\n| --- | ---: |\n")

	if s.SnapshotOK {
		fmt.Fprintf(&b, "| Pending (in sheet now) | %d |\n", s.Pending)
		fmt.Fprintf(&b, "| Done (in sheet now) | %d |\n", s.Done)
		fmt.Fprintf(&b, "| Done, awaiting notification | %d |\n", s.AwaitingNotification)
	} else {
		b.WriteString("| Sheet totals | unavailable |\n")
	}

	fmt.Fprintf(&b, "| Telegram messages fetched | %d |\n", s.UpdatesFetched)
	fmt.Fprintf(&b, "| New referrals recorded | %d |\n", s.NewReferrals)
	fmt.Fprintf(&b, "| Rejected: duplicate | %d |\n", s.Duplicates)
	fmt.Fprintf(&b, "| Rejected: rate limited | %d |\n", s.RateLimited)
	fmt.Fprintf(&b, "| Rejected: unreadable | %d |\n", s.Malformed)
	fmt.Fprintf(&b, "| Done and notified this run | %d |\n", s.Notified)
	fmt.Fprintf(&b, "| Notification failed permanently | %d |\n", s.NotifyPermanentFailures)
	fmt.Fprintf(&b, "| Notification will retry | %d |\n", s.NotifyRetries)

	return b.String()
}

// Digest renders the Telegram message for the owner. Plain text, no parse mode, and it never
// includes candidate names or links, so nothing in it needs escaping.
func (s RunSummary) Digest(runFailed bool) string {
	var b strings.Builder

	b.WriteString("Referral inbox digest\n\n")

	if s.SnapshotOK {
		fmt.Fprintf(&b, "Pending: %d\n", s.Pending)
	} else {
		b.WriteString("Pending: unknown (could not read the sheet)\n")
	}

	fmt.Fprintf(&b, "New this run: %d\n", s.NewReferrals)
	fmt.Fprintf(&b, "Marked done and notified this run: %d\n", s.Notified)

	if s.rejected() > 0 {
		fmt.Fprintf(&b, "Rejected: %d (duplicate %d, rate limited %d, unreadable %d)\n",
			s.rejected(), s.Duplicates, s.RateLimited, s.Malformed)
	}

	if s.deliveryProblems() > 0 {
		fmt.Fprintf(&b, "Notification problems: %d permanent, %d will retry\n",
			s.NotifyPermanentFailures, s.NotifyRetries)
	}

	if runFailed {
		b.WriteString("\nThis run finished with errors. Check the workflow log.")
	}

	return strings.TrimRight(b.String(), "\n")
}

// ShouldSendDigest decides whether a digest is worth sending. An all-quiet run with nothing
// pending would just be a "0 pending" ping every six hours, so it is skipped.
func (s RunSummary) ShouldSendDigest(runFailed bool) bool {
	if runFailed || s.HasActivity() {
		return true
	}

	// If the sheet could not be read the pending count is unknown, which is itself worth
	// surfacing; otherwise stay quiet when there is nothing outstanding.
	return !s.SnapshotOK || s.Pending > 0
}

// SendDigest messages the owner the run summary. It is deliberately best effort: a failed
// digest must never turn an otherwise healthy referral run red, so errors are returned for the
// caller to log rather than treated as a run failure.
func (op *ReferralOperation) SendDigest(ctx context.Context, chatID int64, runFailed bool) error {
	if !op.Summary.ShouldSendDigest(runFailed) {
		log.Println("Nothing pending and nothing happened, skipping digest")
		return nil
	}

	return op.Bot.SendMessage(ctx, chatID, op.Summary.Digest(runFailed))
}

// WriteStepSummary appends the summary to the GitHub Actions run page. It does nothing outside
// Actions, where GITHUB_STEP_SUMMARY is not set.
func WriteStepSummary(summary RunSummary) {
	path := os.Getenv("GITHUB_STEP_SUMMARY")
	if path == "" {
		return
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		log.Printf("Could not open step summary file: %v", err)
		return
	}
	defer file.Close()

	if _, err := file.WriteString(summary.Markdown()); err != nil {
		log.Printf("Could not write step summary: %v", err)
	}
}
