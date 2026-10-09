package configs

import "testing"

func TestParseOwnerChatID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{"unset means digest disabled", "", 0, false},
		{"whitespace only means disabled", "   ", 0, false},
		{"private chat id", "921265420", 921265420, false},
		{"padded", "  921265420  ", 921265420, false},
		{"group chat ids are negative", "-1001234567890", -1001234567890, false},
		{"username is rejected", "@devid", 0, true},
		{"garbage is rejected", "abc", 0, true},
		{"zero is rejected", "0", 0, true},
		{"decimal is rejected", "12.5", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseOwnerChatID(tc.input)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseOwnerChatID(%q) = %d, want an error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseOwnerChatID(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("parseOwnerChatID(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

// A mistyped OWNER_CHAT_ID must fail validation rather than silently disabling the digest.
func TestValidateForInboxRejectsBadOwnerChatID(t *testing.T) {
	t.Setenv("BOT_TOKEN", "token")
	t.Setenv("SPREADSHEET_ID", "sheet")
	t.Setenv("GOOGLE_SA_JSON_B64", "creds")
	t.Setenv("OWNER_CHAT_ID", "@devid")

	if err := InitConfig().ValidateForInbox(); err == nil {
		t.Error("expected a validation error for a non-numeric OWNER_CHAT_ID")
	}
}

func TestValidateForInboxAcceptsMissingOwnerChatID(t *testing.T) {
	t.Setenv("BOT_TOKEN", "token")
	t.Setenv("SPREADSHEET_ID", "sheet")
	t.Setenv("GOOGLE_SA_JSON_B64", "creds")
	t.Setenv("OWNER_CHAT_ID", "")

	if err := InitConfig().ValidateForInbox(); err != nil {
		t.Errorf("OWNER_CHAT_ID is optional, got error: %v", err)
	}
}
