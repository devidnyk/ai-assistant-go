package operations

import (
	"errors"
	"testing"
)

func TestParseReferCommand(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantName  string
		wantJobID string
		wantURL   string
		wantErr   bool
	}{
		{
			name:      "happy path",
			input:     "/refer Jane Doe, JOB-123, https://example.com/cv.pdf",
			wantName:  "Jane Doe",
			wantJobID: "JOB-123",
			wantURL:   "https://example.com/cv.pdf",
		},
		{
			name:      "name containing a comma is preserved",
			input:     "/refer Doe, Jane, JOB-123, https://example.com/cv.pdf",
			wantName:  "Doe, Jane",
			wantJobID: "JOB-123",
			wantURL:   "https://example.com/cv.pdf",
		},
		{
			name:      "botname suffix is stripped",
			input:     "/refer@MyReferralBot Jane Doe, JOB-123, https://example.com/cv.pdf",
			wantName:  "Jane Doe",
			wantJobID: "JOB-123",
			wantURL:   "https://example.com/cv.pdf",
		},
		{
			name:      "extra whitespace is trimmed",
			input:     "   /refer    Jane Doe ,  JOB-123 ,  https://example.com/cv.pdf   ",
			wantName:  "Jane Doe",
			wantJobID: "JOB-123",
			wantURL:   "https://example.com/cv.pdf",
		},
		{
			name:      "query string in url is kept",
			input:     "/refer Jane Doe, JOB-123, https://example.com/cv?id=7&v=2",
			wantName:  "Jane Doe",
			wantJobID: "JOB-123",
			wantURL:   "https://example.com/cv?id=7&v=2",
		},
		{name: "missing third field", input: "/refer Jane Doe, JOB-123", wantErr: true},
		{name: "no commas at all", input: "/refer Jane Doe", wantErr: true},
		{name: "empty body", input: "/refer", wantErr: true},
		{name: "empty candidate name", input: "/refer , JOB-123, https://example.com/cv.pdf", wantErr: true},
		{name: "empty job id", input: "/refer Jane Doe, , https://example.com/cv.pdf", wantErr: true},
		{name: "empty resume link", input: "/refer Jane Doe, JOB-123, ", wantErr: true},
		{name: "javascript scheme rejected", input: "/refer Jane Doe, JOB-123, javascript:alert(1)", wantErr: true},
		{name: "file scheme rejected", input: "/refer Jane Doe, JOB-123, file:///etc/passwd", wantErr: true},
		{name: "relative url rejected", input: "/refer Jane Doe, JOB-123, /resumes/cv.pdf", wantErr: true},
		{name: "scheme without host rejected", input: "/refer Jane Doe, JOB-123, https://", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseReferCommand(tc.input)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %+v", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.CandidateName != tc.wantName {
				t.Errorf("candidate name = %q, want %q", got.CandidateName, tc.wantName)
			}
			if got.JobID != tc.wantJobID {
				t.Errorf("job id = %q, want %q", got.JobID, tc.wantJobID)
			}
			if got.ResumeURL != tc.wantURL {
				t.Errorf("resume url = %q, want %q", got.ResumeURL, tc.wantURL)
			}
		})
	}
}

func TestParseReferCommandRejectsOtherCommands(t *testing.T) {
	for _, input := range []string{"/help", "/start", "hello there", ""} {
		if _, err := ParseReferCommand(input); !errors.Is(err, ErrNotReferCommand) {
			t.Errorf("ParseReferCommand(%q) error = %v, want ErrNotReferCommand", input, err)
		}
	}
}

func TestParseReferCommandEnforcesLengthLimits(t *testing.T) {
	longName := make([]byte, maxCandidateNameLen+1)
	for i := range longName {
		longName[i] = 'a'
	}

	if _, err := ParseReferCommand("/refer " + string(longName) + ", JOB-1, https://example.com/cv.pdf"); err == nil {
		t.Error("expected an error for an over-long candidate name")
	}
}

func TestCommandOf(t *testing.T) {
	tests := map[string]string{
		"/refer Jane, J1, https://e.com": "/refer",
		"/refer@MyBot Jane":              "/refer",
		"/HELP":                          "/help",
		"  /start  ":                     "/start",
		"not a command":                  "",
		"":                               "",
	}

	for input, want := range tests {
		if got := CommandOf(input); got != want {
			t.Errorf("CommandOf(%q) = %q, want %q", input, got, want)
		}
	}
}
