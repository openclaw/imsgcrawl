package messages

import (
	"context"
	"database/sql"
	"errors"
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

func Status(ctx context.Context, path string) (StatusReport, error) {
	snap, err := SnapshotPathContext(ctx, path)
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

func openSnapshot(ctx context.Context, path string) (*store.Store, error) {
	st, err := store.OpenReadOnly(ctx, path)
	if err != nil {
		return nil, err
	}
	if err := requireTables(ctx, st.DB(), "handle", "chat", "chat_handle_join", "message"); err != nil {
		_ = st.Close()
		return nil, err
	}
	return st, nil
}

func requireTables(ctx context.Context, db *sql.DB, tables ...string) error {
	for _, table := range tables {
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
