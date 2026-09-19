package store

import (
	"ai-assistant/internal/models"
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const (
	// ReferralsSheet holds one row per submitted referral request.
	ReferralsSheet = "referrals"
	// StateSheet holds cron bookkeeping, currently just the Telegram update offset.
	StateSheet = "_state"

	offsetKey = "last_offset"

	// referralRange covers every data row (header excluded) across all 13 columns.
	referralRange = ReferralsSheet + "!A2:M"
	// offsetRange is the single fixed cell pair holding the offset key/value.
	offsetRange = StateSheet + "!A2:B2"

	firstDataRow = 2
)

// Column positions within the referrals sheet. Reordering these breaks existing spreadsheets.
const (
	colUpdateID = iota
	colReceivedAt
	colChatID
	colUserID
	colUsername
	colFirstName
	colCandidateName
	colJobID
	colResumeURL
	colStatus
	colNotes
	colNotifiedAt
	colNotifyError
	columnCount
)

// statusColumnLetter must stay in sync with colStatus (0-indexed 9 == column J).
const statusColumnLetter = "J"

var referralHeaders = []any{
	"update_id", "received_at", "chat_id", "user_id", "username",
	"first_name", "candidate_name", "job_id", "resume_url",
	"status", "notes", "notified_at", "notify_error",
}

var stateHeaders = []any{"key", "value"}

// Row is a Referral paired with its 1-based spreadsheet row number, which is required to
// write the notification stamp back to the correct row later.
type Row struct {
	Index int
	models.Referral
}

type SheetStore struct {
	svc           *sheets.Service
	spreadsheetID string
}

// NewSheetStore authenticates with a base64-encoded service account key. The spreadsheet must
// be shared with the service account's client_email or every call fails with 403.
func NewSheetStore(ctx context.Context, saJSONBase64 string, spreadsheetID string) (*SheetStore, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(saJSONBase64))
	if err != nil {
		return nil, fmt.Errorf("decode GOOGLE_SA_JSON_B64: %w", err)
	}

	creds, err := google.CredentialsFromJSON(ctx, decoded, sheets.SpreadsheetsScope)
	if err != nil {
		return nil, fmt.Errorf("parse service account credentials: %w", err)
	}

	svc, err := sheets.NewService(ctx, option.WithCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("create sheets service: %w", err)
	}

	return &SheetStore{svc: svc, spreadsheetID: spreadsheetID}, nil
}

// EnsureSchema creates the referrals and _state tabs, their header rows, and the
// Pending/Done dropdown on the status column if they are not already present.
func (s *SheetStore) EnsureSchema(ctx context.Context) error {
	spreadsheet, err := s.svc.Spreadsheets.Get(s.spreadsheetID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("read spreadsheet metadata: %w", err)
	}

	existing := make(map[string]bool, len(spreadsheet.Sheets))
	for _, sheet := range spreadsheet.Sheets {
		existing[sheet.Properties.Title] = true
	}

	var requests []*sheets.Request
	for _, title := range []string{ReferralsSheet, StateSheet} {
		if !existing[title] {
			requests = append(requests, &sheets.Request{
				AddSheet: &sheets.AddSheetRequest{
					Properties: &sheets.SheetProperties{Title: title},
				},
			})
		}
	}

	if len(requests) > 0 {
		_, err := s.svc.Spreadsheets.BatchUpdate(s.spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
			Requests: requests,
		}).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("create missing sheets: %w", err)
		}
	}

	if err := s.ensureHeader(ctx, ReferralsSheet+"!A1:M1", referralHeaders); err != nil {
		return err
	}
	if err := s.ensureHeader(ctx, StateSheet+"!A1:B1", stateHeaders); err != nil {
		return err
	}

	return s.ensureStatusDropdown(ctx)
}

func (s *SheetStore) ensureHeader(ctx context.Context, rangeA1 string, headers []any) error {
	resp, err := s.svc.Spreadsheets.Values.Get(s.spreadsheetID, rangeA1).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("read header %s: %w", rangeA1, err)
	}

	if len(resp.Values) > 0 && len(resp.Values[0]) > 0 {
		return nil
	}

	_, err = s.svc.Spreadsheets.Values.Update(s.spreadsheetID, rangeA1, &sheets.ValueRange{
		Values: [][]any{headers},
	}).ValueInputOption("RAW").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("write header %s: %w", rangeA1, err)
	}

	return nil
}

// ensureStatusDropdown constrains the status column to Pending/Done so a typo during manual
// editing cannot silently prevent the completion notification from firing.
func (s *SheetStore) ensureStatusDropdown(ctx context.Context) error {
	sheetID, err := s.sheetID(ctx, ReferralsSheet)
	if err != nil {
		return err
	}

	_, err = s.svc.Spreadsheets.BatchUpdate(s.spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{{
			SetDataValidation: &sheets.SetDataValidationRequest{
				Range: &sheets.GridRange{
					SheetId:          sheetID,
					StartRowIndex:    1,
					StartColumnIndex: colStatus,
					EndColumnIndex:   colStatus + 1,
				},
				Rule: &sheets.DataValidationRule{
					Condition: &sheets.BooleanCondition{
						Type: "ONE_OF_LIST",
						Values: []*sheets.ConditionValue{
							{UserEnteredValue: models.StatusPending},
							{UserEnteredValue: models.StatusDone},
						},
					},
					ShowCustomUi: true,
					Strict:       true,
				},
			},
		}},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("apply status validation: %w", err)
	}

	return nil
}

func (s *SheetStore) sheetID(ctx context.Context, title string) (int64, error) {
	spreadsheet, err := s.svc.Spreadsheets.Get(s.spreadsheetID).Context(ctx).Do()
	if err != nil {
		return 0, fmt.Errorf("read spreadsheet metadata: %w", err)
	}

	for _, sheet := range spreadsheet.Sheets {
		if sheet.Properties.Title == title {
			return sheet.Properties.SheetId, nil
		}
	}

	return 0, fmt.Errorf("sheet %q not found", title)
}

// LoadAll returns every referral row together with its spreadsheet row number.
func (s *SheetStore) LoadAll(ctx context.Context) ([]Row, error) {
	resp, err := s.svc.Spreadsheets.Values.Get(s.spreadsheetID, referralRange).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("read referrals: %w", err)
	}

	rows := make([]Row, 0, len(resp.Values))
	for i, raw := range resp.Values {
		if isBlank(raw) {
			continue
		}

		receivedAt, _ := time.Parse(time.RFC3339, cell(raw, colReceivedAt))

		rows = append(rows, Row{
			Index: firstDataRow + i,
			Referral: models.Referral{
				UpdateID:      parseInt64(cell(raw, colUpdateID)),
				ReceivedAt:    receivedAt,
				ChatID:        parseInt64(cell(raw, colChatID)),
				UserID:        parseInt64(cell(raw, colUserID)),
				Username:      cell(raw, colUsername),
				FirstName:     cell(raw, colFirstName),
				CandidateName: cell(raw, colCandidateName),
				JobID:         cell(raw, colJobID),
				ResumeURL:     cell(raw, colResumeURL),
				Status:        cell(raw, colStatus),
				Notes:         cell(raw, colNotes),
				NotifiedAt:    cell(raw, colNotifiedAt),
				NotifyError:   cell(raw, colNotifyError),
			},
		})
	}

	return rows, nil
}

// AppendReferrals adds new rows to the bottom of the referrals sheet.
func (s *SheetStore) AppendReferrals(ctx context.Context, referrals []models.Referral) error {
	if len(referrals) == 0 {
		return nil
	}

	values := make([][]any, 0, len(referrals))
	for _, r := range referrals {
		row := make([]any, columnCount)
		row[colUpdateID] = strconv.FormatInt(r.UpdateID, 10)
		row[colReceivedAt] = r.ReceivedAt.UTC().Format(time.RFC3339)
		row[colChatID] = strconv.FormatInt(r.ChatID, 10)
		row[colUserID] = strconv.FormatInt(r.UserID, 10)
		row[colUsername] = sanitizeCell(r.Username)
		row[colFirstName] = sanitizeCell(r.FirstName)
		row[colCandidateName] = sanitizeCell(r.CandidateName)
		row[colJobID] = sanitizeCell(r.JobID)
		row[colResumeURL] = sanitizeCell(r.ResumeURL)
		row[colStatus] = r.Status
		row[colNotes] = sanitizeCell(r.Notes)
		row[colNotifiedAt] = ""
		row[colNotifyError] = ""
		values = append(values, row)
	}

	_, err := s.svc.Spreadsheets.Values.
		Append(s.spreadsheetID, ReferralsSheet+"!A1", &sheets.ValueRange{Values: values}).
		ValueInputOption("RAW").
		InsertDataOption("INSERT_ROWS").
		Context(ctx).
		Do()
	if err != nil {
		return fmt.Errorf("append referrals: %w", err)
	}

	return nil
}

// MarkNotified stamps the notification timestamp (and optional permanent failure reason) on a
// single row. Writing per row keeps a mid-run crash from losing more than one stamp.
func (s *SheetStore) MarkNotified(ctx context.Context, rowIndex int, notifiedAt time.Time, notifyErr string) error {
	rangeA1 := fmt.Sprintf("%s!L%d:M%d", ReferralsSheet, rowIndex, rowIndex)

	_, err := s.svc.Spreadsheets.Values.Update(s.spreadsheetID, rangeA1, &sheets.ValueRange{
		Values: [][]any{{notifiedAt.UTC().Format(time.RFC3339), sanitizeCell(notifyErr)}},
	}).ValueInputOption("RAW").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("mark row %d notified: %w", rowIndex, err)
	}

	return nil
}

// GetOffset returns the next Telegram update offset to request. Zero means "from the start".
func (s *SheetStore) GetOffset(ctx context.Context) (int64, error) {
	resp, err := s.svc.Spreadsheets.Values.Get(s.spreadsheetID, offsetRange).Context(ctx).Do()
	if err != nil {
		return 0, fmt.Errorf("read offset: %w", err)
	}

	if len(resp.Values) == 0 || len(resp.Values[0]) < 2 {
		return 0, nil
	}

	return parseInt64(cell(resp.Values[0], 1)), nil
}

// SetOffset persists the next offset. It must only be called after the corresponding rows have
// been durably written, because advancing it makes Telegram discard those updates for good.
func (s *SheetStore) SetOffset(ctx context.Context, offset int64) error {
	_, err := s.svc.Spreadsheets.Values.Update(s.spreadsheetID, offsetRange, &sheets.ValueRange{
		Values: [][]any{{offsetKey, strconv.FormatInt(offset, 10)}},
	}).ValueInputOption("RAW").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("write offset: %w", err)
	}

	return nil
}

// sanitizeCell neutralises spreadsheet formula injection. Values are written with the RAW
// input option so Sheets itself will not evaluate them, but a leading =, +, - or @ would still
// execute if the sheet were exported to CSV and opened in Excel.
func sanitizeCell(value string) string {
	if value == "" {
		return value
	}

	switch value[0] {
	case '=', '+', '-', '@':
		return "'" + value
	default:
		return value
	}
}

func cell(row []any, index int) string {
	if index >= len(row) || row[index] == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(row[index]))
}

func isBlank(row []any) bool {
	for i := range row {
		if cell(row, i) != "" {
			return false
		}
	}
	return true
}

func parseInt64(value string) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}
