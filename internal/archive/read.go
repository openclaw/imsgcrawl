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

func (s *Store) CountChats(ctx context.Context) (int64, error) {
	return countTable(ctx, s.store.DB(), "chats")
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
  coalesce(h.handle, ''),
  m.date,
  coalesce(m.service, ''),
  m.is_from_me,
  coalesce(m.text, ''),
  m.has_attachments,
  coalesce(c.display_name, ''),
  coalesce(pc.participants, 0)
from chat_messages cm
join messages m on m.source_rowid = cm.message_rowid
left join handles h on h.source_rowid = m.handle_rowid
left join chats c on c.source_rowid = cm.chat_rowid
left join (
  select chat_rowid, count(distinct handle_rowid) as participants
  from chat_participants
  group by chat_rowid
) pc on pc.chat_rowid = cm.chat_rowid
where cm.chat_rowid = ?
order by m.date %s, m.source_rowid %s
`+limitClause, order, tie), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanMessages(rows)
}

func (s *Store) CountMessages(ctx context.Context, chatID string) (int64, error) {
	id, err := parseID(chatID, "chat")
	if err != nil {
		return 0, err
	}
	var count int64
	err = s.store.DB().QueryRowContext(ctx, `select count(*) from chat_messages where chat_rowid = ?`, id).Scan(&count)
	return count, err
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
  coalesce(nullif(trim(c.display_name), ''), nullif(trim(c.room_name), ''), nullif(trim(c.chat_identifier), ''), c.guid, ''),
  case
    when coalesce(pc.participants, 0) > 1 or nullif(trim(c.room_name), '') is not null then 'group'
    when cm.chat_rowid is null then ''
    else 'direct'
  end,
  coalesce(pc.participants, 0),
  m.handle_rowid,
  coalesce(h.handle, ''),
  m.date,
  coalesce(m.service, ''),
  m.is_from_me,
  m.has_attachments,
  coalesce(m.text, ''),
  coalesce(c.display_name, ''),
  coalesce(pc.participants, 0),
  snippet(messages_fts, 1, '[', ']', '...', 12)
from messages_fts
join messages m on m.source_rowid = messages_fts.source_rowid
left join chat_messages cm on cm.message_rowid = m.source_rowid
left join handles h on h.source_rowid = m.handle_rowid
left join chats c on c.source_rowid = cm.chat_rowid
left join (
  select chat_rowid, count(distinct handle_rowid) as participants
  from chat_participants
  group by chat_rowid
) pc on pc.chat_rowid = cm.chat_rowid
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
		var participantCount int64
		var fromMe, hasAttachments int
		var senderHandle, chatDisplayName string
		var result SearchResult
		if err := rows.Scan(&messageID, &result.GUID, &chatIDValue, &result.ChatTitle, &result.ChatKind, &result.ChatParticipantCount, &handleID, &senderHandle, &result.Date, &result.Service, &fromMe, &hasAttachments, &result.Text, &chatDisplayName, &participantCount, &result.Snippet); err != nil {
			return nil, err
		}
		result.MessageID = strconv.FormatInt(messageID, 10)
		if chatIDValue != 0 {
			result.ChatID = strconv.FormatInt(chatIDValue, 10)
		}
		if handleID != 0 {
			result.HandleID = strconv.FormatInt(handleID, 10)
		}
		result.SenderHandle = senderHandle
		result.FromMe = fromMe != 0
		result.HasAttachments = hasAttachments != 0
		result.SenderLabel = senderLabel(result.FromMe, senderHandle, chatDisplayName, participantCount)
		out = append(out, result)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if out[i].ChatID == "" {
			continue
		}
		handles, err := participantHandles(ctx, s.store.DB(), out[i].ChatID)
		if err != nil {
			return nil, err
		}
		out[i].ChatParticipantHandles = handles
	}
	return out, nil
}

func (s *Store) CountSearch(ctx context.Context, query string) (int64, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return 0, errors.New("search query is required")
	}
	var count int64
	err := s.store.DB().QueryRowContext(ctx, `
select count(*)
from messages_fts
join messages m on m.source_rowid = messages_fts.source_rowid
left join chat_messages cm on cm.message_rowid = m.source_rowid
where messages_fts match ?
`, ftsQuery(query)).Scan(&count)
	return count, err
}

func scanMessages(rows *sql.Rows) ([]MessageRow, error) {
	out := []MessageRow{}
	for rows.Next() {
		var row MessageRow
		var messageID, chatID, handleID int64
		var participantCount int64
		var fromMe, hasAttachments int
		var chatDisplayName string
		if err := rows.Scan(&messageID, &row.GUID, &chatID, &handleID, &row.SenderHandle, &row.Date, &row.Service, &fromMe, &row.Text, &hasAttachments, &chatDisplayName, &participantCount); err != nil {
			return nil, err
		}
		row.MessageID = strconv.FormatInt(messageID, 10)
		row.ChatID = strconv.FormatInt(chatID, 10)
		if handleID != 0 {
			row.HandleID = strconv.FormatInt(handleID, 10)
		}
		row.FromMe = fromMe != 0
		row.HasAttachments = hasAttachments != 0
		row.SenderLabel = senderLabel(row.FromMe, row.SenderHandle, chatDisplayName, participantCount)
		out = append(out, row)
	}
	return out, rows.Err()
}

func senderLabel(fromMe bool, handle, chatDisplayName string, participantCount int64) string {
	if fromMe {
		return "me"
	}
	if participantCount <= 1 {
		if display := strings.TrimSpace(chatDisplayName); display != "" {
			return display
		}
	}
	if handle = strings.TrimSpace(handle); handle != "" {
		return handle
	}
	return "them"
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
