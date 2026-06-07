package archive

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func (s *Store) Status(ctx context.Context) (Status, error) {
	status := Status{ArchivePath: s.path, ArchiveBytes: fileSize(s.path)}
	state, err := s.syncState(ctx)
	if err != nil {
		return Status{}, err
	}
	status.LastSyncAt = state["last_sync_at"]
	status.SourcePath = state["source_path"]
	status.SourceModifiedAt = state["source_modified_at"]
	if sourceBytes := state["source_bytes"]; sourceBytes != "" {
		status.SourceBytes, _ = strconv.ParseInt(sourceBytes, 10, 64)
	}
	db := s.store.DB()
	if status.Handles, err = countTable(ctx, db, "handles"); err != nil {
		return Status{}, err
	}
	if status.Chats, err = countTable(ctx, db, "chats"); err != nil {
		return Status{}, err
	}
	if status.Participants, err = countTable(ctx, db, "chat_participants"); err != nil {
		return Status{}, err
	}
	if status.ChatMessages, err = countTable(ctx, db, "chat_messages"); err != nil {
		return Status{}, err
	}
	if status.Messages, err = countTable(ctx, db, "messages"); err != nil {
		return Status{}, err
	}
	_ = db.QueryRowContext(ctx, `select coalesce(max(date), 0) from messages`).Scan(&status.LatestMessageDate)
	return status, nil
}

func (s *Store) Chats(ctx context.Context, limit int) ([]ChatSummary, error) {
	limitClause := ""
	args := []any{}
	if limit > 0 {
		limitClause = "limit ?"
		args = append(args, limit)
	}
	rows, err := s.store.DB().QueryContext(ctx, `
select
  c.source_rowid,
  c.guid,
  coalesce(nullif(trim(c.display_name), ''), nullif(trim(c.chat_identifier), ''), c.guid) as title,
  coalesce(c.chat_identifier, ''),
  coalesce(c.service_name, ''),
  count(distinct cp.handle_rowid) as participants,
  count(cm.message_rowid) as messages,
  coalesce(max(m.date), 0) as latest_message
from chats c
left join chat_participants cp on cp.chat_rowid = c.source_rowid
left join chat_messages cm on cm.chat_rowid = c.source_rowid
left join messages m on m.source_rowid = cm.message_rowid
group by c.source_rowid, c.guid, c.display_name, c.chat_identifier, c.service_name
order by latest_message desc, c.source_rowid desc
`+limitClause, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []ChatSummary{}
	for rows.Next() {
		var c ChatSummary
		var chatID int64
		if err := rows.Scan(&chatID, &c.GUID, &c.Title, &c.ChatIdentifier, &c.Service, &c.ParticipantCount, &c.MessageCount, &c.LatestMessageDate); err != nil {
			return nil, err
		}
		c.ChatID = strconv.FormatInt(chatID, 10)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) Messages(ctx context.Context, chatID string, limit int, asc bool) ([]MessageRow, error) {
	id, err := parseID(chatID, "chat")
	if err != nil {
		return nil, err
	}
	order := "desc"
	tie := "desc"
	if asc {
		order = "asc"
		tie = "asc"
	}
	limitClause := ""
	args := []any{id}
	if limit > 0 {
		limitClause = "limit ?"
		args = append(args, limit)
	}
	rows, err := s.store.DB().QueryContext(ctx, fmt.Sprintf(`
select
  m.source_rowid,
  m.guid,
  cm.chat_rowid,
  m.handle_rowid,
  m.date,
  coalesce(m.service, ''),
  m.is_from_me,
  coalesce(m.text, ''),
  m.has_attachments
from chat_messages cm
join messages m on m.source_rowid = cm.message_rowid
where cm.chat_rowid = ?
order by m.date %s, m.source_rowid %s
`+limitClause, order, tie), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanMessages(rows)
}

func (s *Store) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("search query is required")
	}
	limitClause := ""
	args := []any{ftsQuery(query)}
	if limit > 0 {
		limitClause = "limit ?"
		args = append(args, limit)
	}
	rows, err := s.store.DB().QueryContext(ctx, `
select
  m.source_rowid,
  m.guid,
  coalesce(cm.chat_rowid, 0),
  m.handle_rowid,
  m.date,
  coalesce(m.service, ''),
  m.is_from_me,
  m.has_attachments,
  snippet(messages_fts, 1, '[', ']', '...', 12)
from messages_fts
join messages m on m.source_rowid = messages_fts.source_rowid
left join chat_messages cm on cm.message_rowid = m.source_rowid
where messages_fts match ?
order by rank, cm.chat_rowid
`+limitClause, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []SearchResult{}
	for rows.Next() {
		var messageID, chatIDValue, handleID int64
		var fromMe, hasAttachments int
		var result SearchResult
		if err := rows.Scan(&messageID, &result.GUID, &chatIDValue, &handleID, &result.Date, &result.Service, &fromMe, &hasAttachments, &result.Snippet); err != nil {
			return nil, err
		}
		result.MessageID = strconv.FormatInt(messageID, 10)
		if chatIDValue != 0 {
			result.ChatID = strconv.FormatInt(chatIDValue, 10)
		}
		if handleID != 0 {
			result.HandleID = strconv.FormatInt(handleID, 10)
		}
		result.FromMe = fromMe != 0
		result.HasAttachments = hasAttachments != 0
		out = append(out, result)
	}
	return out, rows.Err()
}

func scanMessages(rows *sql.Rows) ([]MessageRow, error) {
	out := []MessageRow{}
	for rows.Next() {
		var row MessageRow
		var messageID, chatID, handleID int64
		var fromMe, hasAttachments int
		if err := rows.Scan(&messageID, &row.GUID, &chatID, &handleID, &row.Date, &row.Service, &fromMe, &row.Text, &hasAttachments); err != nil {
			return nil, err
		}
		row.MessageID = strconv.FormatInt(messageID, 10)
		row.ChatID = strconv.FormatInt(chatID, 10)
		if handleID != 0 {
			row.HandleID = strconv.FormatInt(handleID, 10)
		}
		row.FromMe = fromMe != 0
		row.HasAttachments = hasAttachments != 0
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) syncState(ctx context.Context) (map[string]string, error) {
	rows, err := s.store.DB().QueryContext(ctx, `select key, value from sync_state`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		out[key] = value
	}
	return out, rows.Err()
}
