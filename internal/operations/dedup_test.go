package operations

import "testing"

func TestDedupKeyMatchesEquivalentSubmissions(t *testing.T) {
	base := DedupKey(100, "Jane Doe", "JOB-123")

	equivalent := []struct {
		name          string
		chatID        int64
		candidateName string
		jobID         string
	}{
		{"identical", 100, "Jane Doe", "JOB-123"},
		{"different case", 100, "jane doe", "job-123"},
		{"extra inner whitespace", 100, "Jane  Doe", "JOB-123"},
		{"surrounding whitespace", 100, "  Jane Doe  ", " JOB-123 "},
		{"tab separated", 100, "Jane\tDoe", "JOB-123"},
	}

	for _, tc := range equivalent {
		t.Run(tc.name, func(t *testing.T) {
			if got := DedupKey(tc.chatID, tc.candidateName, tc.jobID); got != base {
				t.Errorf("DedupKey(%d, %q, %q) = %q, want %q", tc.chatID, tc.candidateName, tc.jobID, got, base)
			}
		})
	}
}

func TestDedupKeyDistinguishesDifferentSubmissions(t *testing.T) {
	base := DedupKey(100, "Jane Doe", "JOB-123")

	different := []struct {
		name          string
		chatID        int64
		candidateName string
		jobID         string
	}{
		{"different chat", 200, "Jane Doe", "JOB-123"},
		{"different candidate", 100, "John Roe", "JOB-123"},
		{"different job", 100, "Jane Doe", "JOB-999"},
	}

	for _, tc := range different {
		t.Run(tc.name, func(t *testing.T) {
			if got := DedupKey(tc.chatID, tc.candidateName, tc.jobID); got == base {
				t.Errorf("DedupKey(%d, %q, %q) collided with %q", tc.chatID, tc.candidateName, tc.jobID, base)
			}
		})
	}
}

// The separator must not let field boundaries be forged, e.g. a candidate name containing the
// delimiter should not be able to impersonate a different candidate/job pair.
func TestDedupKeyFieldsCannotBeForged(t *testing.T) {
	a := DedupKey(100, "Jane|JOB-1", "JOB-2")
	b := DedupKey(100, "Jane", "JOB-1|JOB-2")

	if a == b {
		t.Errorf("keys collided across field boundaries: %q", a)
	}
}
