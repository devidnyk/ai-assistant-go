package configs

import "testing"

func TestNormalizeSpreadsheetID(t *testing.T) {
	const id = "1fjxgrXakwpYkR2f9miZsz2xB5AQ8CYDfezcwhEa9rfM"

	tests := map[string]string{
		id:               id,
		"  " + id + "  ": id,
		"https://docs.google.com/spreadsheets/d/" + id + "/edit":             id,
		"https://docs.google.com/spreadsheets/d/" + id + "/edit#gid=0":       id,
		"https://docs.google.com/spreadsheets/d/" + id + "/edit?usp=sharing": id,
		"https://docs.google.com/spreadsheets/d/" + id:                       id,
		"": "",
	}

	for input, want := range tests {
		if got := normalizeSpreadsheetID(input); got != want {
			t.Errorf("normalizeSpreadsheetID(%q) = %q, want %q", input, got, want)
		}
	}
}
