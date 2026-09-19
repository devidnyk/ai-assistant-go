package models

import (
	"strings"
	"time"
)

// Referral status values. The bot only ever writes StatusPending; StatusDone is set by hand
// in the spreadsheet once the referral has actually been submitted.
const (
	StatusPending = "Pending"
	StatusDone    = "Done"
)

// Referral mirrors one row of the referrals sheet.
type Referral struct {
	UpdateID      int64
	ReceivedAt    time.Time
	ChatID        int64
	UserID        int64
	Username      string
	FirstName     string
	CandidateName string
	JobID         string
	ResumeURL     string
	Status        string
	Notes         string
	NotifiedAt    string
	NotifyError   string
}

// NormalizeStatus makes manual edits robust: "done", " Done " and "DONE" all count as done.
func NormalizeStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func (r Referral) IsDone() bool {
	return NormalizeStatus(r.Status) == NormalizeStatus(StatusDone)
}

// IsNotified reports whether the completion message has already been delivered. This is the
// sole idempotency guard preventing the cron job from re-notifying on every run.
func (r Referral) IsNotified() bool {
	return strings.TrimSpace(r.NotifiedAt) != ""
}
