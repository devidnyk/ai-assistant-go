package operations

import (
	"ai-assistant/internal/store"
	"strings"
	"testing"
)

func row(status string, notifiedAt string) store.Row {
	var r store.Row
	r.Status = status
	r.NotifiedAt = notifiedAt
	return r
}

func TestTallyRows(t *testing.T) {
	rows := []store.Row{
		row("Pending", ""),
		row("Pending", ""),
		row("", ""),                      // blank status still counts as outstanding
		row("typo", ""),                  // so does a mistyped one
		row("Done", "2026-10-09T10:00Z"), // done and notified
		row("done", ""),                  // done, case-insensitive, awaiting notification
		row(" Done ", ""),                // done, padded, awaiting notification
	}

	pending, done, awaiting := tallyRows(rows)

	if pending != 4 {
		t.Errorf("pending = %d, want 4", pending)
	}
	if done != 3 {
		t.Errorf("done = %d, want 3", done)
	}
	if awaiting != 2 {
		t.Errorf("awaiting notification = %d, want 2", awaiting)
	}
}

func TestTallyRowsEmpty(t *testing.T) {
	pending, done, awaiting := tallyRows(nil)

	if pending != 0 || done != 0 || awaiting != 0 {
		t.Errorf("tallyRows(nil) = %d, %d, %d, want all zero", pending, done, awaiting)
	}
}

func TestShouldSendDigest(t *testing.T) {
	tests := []struct {
		name      string
		summary   RunSummary
		runFailed bool
		want      bool
	}{
		{"quiet run with nothing pending is skipped", RunSummary{SnapshotOK: true}, false, false},
		{"pending work is reported", RunSummary{SnapshotOK: true, Pending: 3}, false, true},
		{"new referral is reported", RunSummary{SnapshotOK: true, NewReferrals: 1}, false, true},
		{"notification sent is reported", RunSummary{SnapshotOK: true, Notified: 1}, false, true},
		{"rejection is reported", RunSummary{SnapshotOK: true, RateLimited: 1}, false, true},
		{"delivery problem is reported", RunSummary{SnapshotOK: true, NotifyRetries: 1}, false, true},
		{"failed run is always reported", RunSummary{SnapshotOK: true}, true, true},
		{"unreadable sheet is reported", RunSummary{SnapshotOK: false}, false, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.summary.ShouldSendDigest(tc.runFailed); got != tc.want {
				t.Errorf("ShouldSendDigest = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDigestReportsPendingCount(t *testing.T) {
	digest := RunSummary{SnapshotOK: true, Pending: 7, NewReferrals: 2, Notified: 1}.Digest(false)

	for _, want := range []string{"Pending: 7", "New this run: 2", "Marked done and notified this run: 1"} {
		if !strings.Contains(digest, want) {
			t.Errorf("digest missing %q:\n%s", want, digest)
		}
	}
	if strings.Contains(digest, "Rejected") {
		t.Errorf("digest should omit the rejected line when nothing was rejected:\n%s", digest)
	}
	if strings.Contains(digest, "errors") {
		t.Errorf("digest should not mention errors on a healthy run:\n%s", digest)
	}
}

func TestDigestSaysUnknownRatherThanZeroWhenSheetUnreadable(t *testing.T) {
	digest := RunSummary{SnapshotOK: false}.Digest(false)

	if !strings.Contains(digest, "Pending: unknown") {
		t.Errorf("digest must not report a misleading zero:\n%s", digest)
	}
	if strings.Contains(digest, "Pending: 0") {
		t.Errorf("digest reported Pending: 0 for an unreadable sheet:\n%s", digest)
	}
}

func TestDigestIncludesRejectionsAndFailures(t *testing.T) {
	digest := RunSummary{
		SnapshotOK:              true,
		Duplicates:              1,
		RateLimited:             2,
		Malformed:               3,
		NotifyPermanentFailures: 1,
		NotifyRetries:           2,
	}.Digest(true)

	for _, want := range []string{
		"Rejected: 6 (duplicate 1, rate limited 2, unreadable 3)",
		"Notification problems: 1 permanent, 2 will retry",
		"finished with errors",
	} {
		if !strings.Contains(digest, want) {
			t.Errorf("digest missing %q:\n%s", want, digest)
		}
	}
}

// The digest is plain text with no parse mode, and must never carry candidate details.
func TestDigestContainsNoPersonalData(t *testing.T) {
	digest := RunSummary{SnapshotOK: true, Pending: 1, NewReferrals: 1}.Digest(false)

	for _, forbidden := range []string{"http", "JOB-", "@"} {
		if strings.Contains(digest, forbidden) {
			t.Errorf("digest unexpectedly contains %q:\n%s", forbidden, digest)
		}
	}
}

func TestLogLineIsSingleLineAndGreppable(t *testing.T) {
	line := RunSummary{
		SnapshotOK: true, UpdatesFetched: 4, NewReferrals: 2, Notified: 1,
		Pending: 5, Done: 9, AwaitingNotification: 0,
	}.LogLine()

	if strings.Contains(line, "\n") {
		t.Errorf("log line must be a single line: %q", line)
	}
	for _, want := range []string{"fetched=4", "recorded=2", "notified_done=1", "pending=5", "done=9"} {
		if !strings.Contains(line, want) {
			t.Errorf("log line missing %q: %s", want, line)
		}
	}
}

func TestLogLineWhenSheetUnavailable(t *testing.T) {
	line := RunSummary{SnapshotOK: false}.LogLine()

	if !strings.Contains(line, "sheet: unavailable") {
		t.Errorf("log line should flag the missing snapshot: %s", line)
	}
	if strings.Contains(line, "pending=") {
		t.Errorf("log line should not report pending totals it never read: %s", line)
	}
}

func TestMarkdownIsATable(t *testing.T) {
	md := RunSummary{SnapshotOK: true, Pending: 3, Done: 8}.Markdown()

	for _, want := range []string{"| Metric | Count |", "| Pending (in sheet now) | 3 |", "| Done (in sheet now) | 8 |"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
}
