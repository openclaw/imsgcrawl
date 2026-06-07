package messages

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"

	"github.com/openclaw/crawlkit/control"
	"github.com/openclaw/crawlkit/store"
)

type StatusReport struct {
	SchemaVersion string          `json:"schema_version"`
	AppID         string          `json:"app_id"`
	State         string          `json:"state"`
	Summary       string          `json:"summary"`
	DatabasePath  string          `json:"database_path"`
	DatabaseBytes int64           `json:"database_bytes,omitempty"`
	Handles       int64           `json:"handles"`
	Chats         int64           `json:"chats"`
	Messages      int64           `json:"messages"`
	PhoneHandles  int64           `json:"phone_handles"`
	EmailHandles  int64           `json:"email_handles"`
	OtherHandles  int64           `json:"other_handles"`
	Counts        []control.Count `json:"counts,omitempty"`
	Warnings      []string        `json:"warnings,omitempty"`
}

type ExportedContact struct {
	DisplayName  string   `json:"display_name"`
	PhoneNumbers []string `json:"phone_numbers"`
}

type handleRow struct {
	ID          string
	Service     string
	DisplayName string
	Messages    int64
	LastMessage int64
}

func Status(ctx context.Context, path string) (StatusReport, error) {
	snap, err := SnapshotPath(path)
	if err != nil {
		return StatusReport{}, err
	}
	defer func() { _ = snap.Close() }()
	st, err := openSnapshot(ctx, snap.Path)
	if err != nil {
		return StatusReport{}, err
	}
	defer func() { _ = st.Close() }()
	db := st.DB()
	report := StatusReport{
		SchemaVersion: control.SchemaVersion,
		AppID:         "imsgcrawl",
		State:         "ok",
		Summary:       "Messages database is readable.",
		DatabasePath:  snap.SourcePath,
	}
	report.DatabaseBytes = fileSize(snap.SourcePath)
	report.Handles, err = countTable(ctx, db, "handle")
	if err != nil {
		return StatusReport{}, err
	}
	report.Chats, err = countTable(ctx, db, "chat")
	if err != nil {
		return StatusReport{}, err
	}
	report.Messages, err = countTable(ctx, db, "message")
	if err != nil {
		return StatusReport{}, err
	}
	report.PhoneHandles, report.EmailHandles, report.OtherHandles, err = handleKindCounts(ctx, db)
	if err != nil {
		return StatusReport{}, err
	}
	report.Counts = []control.Count{
		control.NewCount("handles", "Handles", report.Handles),
		control.NewCount("chats", "Chats", report.Chats),
		control.NewCount("messages", "Messages", report.Messages),
		control.NewCount("phone_handles", "Phone handles", report.PhoneHandles),
		control.NewCount("email_handles", "Email handles", report.EmailHandles),
		control.NewCount("other_handles", "Other handles", report.OtherHandles),
	}
	return report, nil
}

func ExportContacts(ctx context.Context, path string) ([]ExportedContact, error) {
	snap, err := SnapshotPath(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = snap.Close() }()
	st, err := openSnapshot(ctx, snap.Path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = st.Close() }()
	rows, err := phoneHandleRows(ctx, st.DB())
	if err != nil {
		return nil, err
	}
	byPhone := map[string]handleRow{}
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		phoneKey := NormalizePhone(row.ID)
		if phoneKey == "" || !LooksPhoneLike(row.ID) {
			continue
		}
		if current, ok := byPhone[phoneKey]; ok {
			if preferHandle(row, current) {
				byPhone[phoneKey] = row
			}
			continue
		}
		byPhone[phoneKey] = row
		order = append(order, phoneKey)
	}
	sort.SliceStable(order, func(i, j int) bool {
		left := byPhone[order[i]]
		right := byPhone[order[j]]
		if left.LastMessage != right.LastMessage {
			return left.LastMessage > right.LastMessage
		}
		return order[i] < order[j]
	})
	out := make([]ExportedContact, 0, len(order))
	for _, key := range order {
		row := byPhone[key]
		name := strings.TrimSpace(row.DisplayName)
		if name == "" {
			name = strings.TrimSpace(row.ID)
		}
		if name == "" {
			continue
		}
		out = append(out, ExportedContact{DisplayName: name, PhoneNumbers: []string{strings.TrimSpace(row.ID)}})
	}
	return out, nil
}

func openSnapshot(ctx context.Context, path string) (*store.Store, error) {
	st, err := store.OpenReadOnly(ctx, path)
	if err != nil {
		return nil, err
	}
	if err := requireMessagesTables(ctx, st.DB()); err != nil {
		_ = st.Close()
		return nil, err
	}
	return st, nil
}

func requireMessagesTables(ctx context.Context, db *sql.DB) error {
	for _, table := range []string{"handle", "chat", "chat_handle_join", "message"} {
		var name string
		err := db.QueryRowContext(ctx, tableExistsSQL, table).Scan(&name)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errors.New("messages database is missing table " + table)
			}
			return err
		}
	}
	return nil
}

func countTable(ctx context.Context, db *sql.DB, table string) (int64, error) {
	var count int64
	if err := db.QueryRowContext(ctx, `select count(*) from `+table).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func handleKindCounts(ctx context.Context, db *sql.DB) (phones, emails, other int64, err error) {
	rows, err := db.QueryContext(ctx, handleIDsSQL)
	if err != nil {
		return 0, 0, 0, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, 0, 0, err
		}
		switch {
		case strings.Contains(id, "@"):
			emails++
		case LooksPhoneLike(id):
			phones++
		default:
			other++
		}
	}
	if err := rows.Err(); err != nil {
		return 0, 0, 0, err
	}
	return phones, emails, other, nil
}

func phoneHandleRows(ctx context.Context, db *sql.DB) ([]handleRow, error) {
	rows, err := db.QueryContext(ctx, phoneHandleRowsSQL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []handleRow
	for rows.Next() {
		var row handleRow
		if err := rows.Scan(&row.ID, &row.Service, &row.DisplayName, &row.Messages, &row.LastMessage); err != nil {
			return nil, err
		}
		if !LooksPhoneLike(row.ID) {
			continue
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func preferHandle(candidate, current handleRow) bool {
	if candidate.LastMessage != current.LastMessage {
		return candidate.LastMessage > current.LastMessage
	}
	if candidate.Messages != current.Messages {
		return candidate.Messages > current.Messages
	}
	if candidate.DisplayName != "" && current.DisplayName == "" {
		return true
	}
	return len([]rune(candidate.DisplayName)) > len([]rune(current.DisplayName))
}

func NormalizePhone(phone string) string {
	var b strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return strings.TrimPrefix(b.String(), "00")
}

func LooksPhoneLike(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	hasDigit := false
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case r == '+', r == ' ', r == '\t', r == '(', r == ')', r == '-', r == '.':
			continue
		default:
			return false
		}
	}
	return hasDigit
}
