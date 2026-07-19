package archive

import (
	"context"
	"database/sql"
	"fmt"

	ckstore "github.com/openclaw/crawlkit/store"
	"github.com/openclaw/imsgcrawl/internal/messages"
)

func migrate(ctx context.Context, inner *ckstore.Store) error {
	current, err := inner.SchemaVersion(ctx)
	if err != nil {
		return err
	}
	if current > schemaVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d", current, schemaVersion)
	}
	if current < 2 {
		if err := migrateTombstones(ctx, inner.DB()); err != nil {
			return err
		}
	}
	return inner.EnsureSchemaVersion(ctx, schemaVersion)
}

func migrateTombstones(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tombstone migration: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	columns := map[string][]string{
		"handles":           {"deleted_at text", "deletion_reason text"},
		"chats":             {"deleted_at text", "deletion_reason text"},
		"chat_participants": {"deleted_at text", "deletion_reason text"},
		"chat_messages":     {"deleted_at text", "deletion_reason text"},
		"messages": {
			"date_edited integer not null default 0", "date_retracted integer not null default 0",
			"revision_data blob", "deleted_at text", "deletion_reason text",
		},
	}
	for table, definitions := range columns {
		for _, definition := range definitions {
			if err := ensureColumn(ctx, tx, table, definition); err != nil {
				return err
			}
		}
	}
	rows, err := tx.QueryContext(ctx, `select source_rowid, guid, handle_rowid, date,
coalesce(service, ''), is_from_me, coalesce(text, ''), has_attachments,
date_edited, date_retracted, revision_data from messages order by source_rowid`)
	if err != nil {
		return fmt.Errorf("read legacy messages: %w", err)
	}
	var legacy []messages.Message
	for rows.Next() {
		var message messages.Message
		var fromMe, attachments int
		if err := rows.Scan(&message.SourceRowID, &message.GUID, &message.HandleRowID, &message.Date,
			&message.Service, &fromMe, &message.Text, &attachments, &message.DateEdited,
			&message.DateRetracted, &message.RevisionData); err != nil {
			_ = rows.Close()
			return err
		}
		message.IsFromMe = fromMe != 0
		message.HasAttachments = attachments != 0
		legacy = append(legacy, message)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("read legacy messages: %w", err)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, message := range legacy {
		if err := appendMessageEvent(ctx, tx, message, unixEpoch); err != nil {
			return fmt.Errorf("seed legacy message event: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tombstone migration: %w", err)
	}
	committed = true
	return nil
}

func ensureColumn(ctx context.Context, tx *sql.Tx, table, definition string) error {
	var name string
	if _, err := fmt.Sscan(definition, &name); err != nil {
		return err
	}
	var exists int
	if err := tx.QueryRowContext(ctx, "select count(*) from pragma_table_info(?) where name = ?", table, name).Scan(&exists); err != nil {
		return fmt.Errorf("inspect %s.%s: %w", table, name, err)
	}
	if exists != 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, "alter table "+ckstore.QuoteIdent(table)+" add column "+definition); err != nil {
		return fmt.Errorf("add %s.%s: %w", table, name, err)
	}
	return nil
}
