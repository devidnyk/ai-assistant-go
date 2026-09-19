package models

import "testing"

func TestReferralIsDone(t *testing.T) {
	tests := map[string]bool{
		"Done":    true,
		"done":    true,
		"  Done ": true,
		"DONE":    true,
		"Pending": false,
		"":        false,
		"Done!":   false,
	}

	for status, want := range tests {
		referral := Referral{Status: status}
		if got := referral.IsDone(); got != want {
			t.Errorf("Referral{Status: %q}.IsDone() = %v, want %v", status, got, want)
		}
	}
}

func TestReferralIsNotified(t *testing.T) {
	tests := map[string]bool{
		"2026-09-20T10:00:00Z": true,
		"":                     false,
		"   ":                  false,
	}

	for notifiedAt, want := range tests {
		referral := Referral{NotifiedAt: notifiedAt}
		if got := referral.IsNotified(); got != want {
			t.Errorf("Referral{NotifiedAt: %q}.IsNotified() = %v, want %v", notifiedAt, got, want)
		}
	}
}

// A Done row that has already been notified must never be picked up again; this pairing is
// the only thing preventing repeat notifications on every cron run.
func TestDoneAndNotifiedIsSkipped(t *testing.T) {
	referral := Referral{Status: StatusDone, NotifiedAt: "2026-09-20T10:00:00Z"}

	if !referral.IsDone() {
		t.Fatal("expected referral to be done")
	}
	if !referral.IsNotified() {
		t.Fatal("expected referral to be already notified")
	}
}
