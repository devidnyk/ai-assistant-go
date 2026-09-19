package store

import "testing"

func TestSanitizeCell(t *testing.T) {
	tests := map[string]string{
		`=HYPERLINK("http://evil.test","click")`: `'=HYPERLINK("http://evil.test","click")`,
		"+1-555-0100":                            "'+1-555-0100",
		"-2+3":                                   "'-2+3",
		"@SUM(A1:A9)":                            "'@SUM(A1:A9)",
		"Jane Doe":                               "Jane Doe",
		"https://example.com/cv.pdf":             "https://example.com/cv.pdf",
		"":                                       "",
	}

	for input, want := range tests {
		if got := sanitizeCell(input); got != want {
			t.Errorf("sanitizeCell(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestCellHandlesRaggedRows(t *testing.T) {
	// Sheets omits trailing empty cells, so reads must tolerate short rows.
	row := []any{"1", nil, "  spaced  "}

	if got := cell(row, 0); got != "1" {
		t.Errorf("cell(row, 0) = %q, want %q", got, "1")
	}
	if got := cell(row, 1); got != "" {
		t.Errorf("cell(row, 1) = %q, want empty", got)
	}
	if got := cell(row, 2); got != "spaced" {
		t.Errorf("cell(row, 2) = %q, want %q", got, "spaced")
	}
	if got := cell(row, 12); got != "" {
		t.Errorf("cell(row, 12) = %q, want empty for an out-of-range index", got)
	}
}

func TestParseInt64(t *testing.T) {
	tests := map[string]int64{
		"42":   42,
		" 42 ": 42,
		"-7":   -7,
		"":     0,
		"abc":  0,
	}

	for input, want := range tests {
		if got := parseInt64(input); got != want {
			t.Errorf("parseInt64(%q) = %d, want %d", input, got, want)
		}
	}
}
