package messages

import (
	"cmp"
	"context"
	"database/sql"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/openclaw/crawlkit/control"
)

type handleRow struct {
	ID          string
	Service     string
	DisplayName string
	Messages    int64
	LastMessage int64
}

func ExportContacts(ctx context.Context, path string) ([]control.Contact, error) {
	snap, err := SnapshotPathContext(ctx, path)
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
		if phoneKey == "" {
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
	slices.SortFunc(order, func(left, right string) int {
		if latest := cmp.Compare(byPhone[right].LastMessage, byPhone[left].LastMessage); latest != 0 {
			return latest
		}
		return strings.Compare(left, right)
	})
	out := make([]control.Contact, 0, len(order))
	for _, key := range order {
		row := byPhone[key]
		name := strings.TrimSpace(row.DisplayName)
		if name == "" {
			name = strings.TrimSpace(row.ID)
		}
		out = append(out, control.Contact{DisplayName: name, PhoneNumbers: []string{strings.TrimSpace(row.ID)}})
	}
	return out, nil
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
	return utf8.RuneCountInString(candidate.DisplayName) > utf8.RuneCountInString(current.DisplayName)
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
