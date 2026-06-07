package archive

import (
	"context"
	"database/sql"
	"strconv"
)

func (s *Store) Chats(ctx context.Context, limit int) ([]ChatSummary, error) {
	db := s.store.DB()
	limitClause := ""
	args := []any{}
	if limit > 0 {
		limitClause = "limit ?"
		args = append(args, limit)
	}
	rows, err := db.QueryContext(ctx, chatSummaryQuery("")+limitClause, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out, err := scanChatSummaries(rows)
	if err != nil {
		return nil, err
	}
	if err := populateParticipantHandles(ctx, db, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) Chat(ctx context.Context, chatID string) (ChatSummary, error) {
	id, err := parseID(chatID, "chat")
	if err != nil {
		return ChatSummary{}, err
	}
	db := s.store.DB()
	rows, err := db.QueryContext(ctx, chatSummaryQuery("where c.source_rowid = ?"), id)
	if err != nil {
		return ChatSummary{}, err
	}
	defer func() { _ = rows.Close() }()
	out, err := scanChatSummaries(rows)
	if err != nil {
		return ChatSummary{}, err
	}
	if len(out) == 0 {
		return ChatSummary{ChatID: chatID, Title: "chat " + chatID, Kind: "unknown"}, nil
	}
	if err := populateParticipantHandles(ctx, db, out); err != nil {
		return ChatSummary{}, err
	}
	return out[0], nil
}

func chatSummaryQuery(where string) string {
	return `
select
  c.source_rowid,
  c.guid,
  coalesce(nullif(trim(c.display_name), ''), nullif(trim(c.room_name), ''), nullif(trim(c.chat_identifier), ''), c.guid) as title,
  case
    when count(distinct cp.handle_rowid) > 1 or nullif(trim(c.room_name), '') is not null then 'group'
    else 'direct'
  end as kind,
  coalesce(c.chat_identifier, ''),
  coalesce(c.room_name, ''),
  coalesce(c.service_name, ''),
  count(distinct cp.handle_rowid) as participants,
  count(distinct cm.message_rowid) as messages,
  coalesce(max(m.date), 0) as latest_message
from chats c
left join chat_participants cp on cp.chat_rowid = c.source_rowid
left join chat_messages cm on cm.chat_rowid = c.source_rowid
left join messages m on m.source_rowid = cm.message_rowid
` + where + `
group by c.source_rowid, c.guid, c.display_name, c.room_name, c.chat_identifier, c.service_name
order by latest_message desc, c.source_rowid desc
`
}

func scanChatSummaries(rows *sql.Rows) ([]ChatSummary, error) {
	out := []ChatSummary{}
	for rows.Next() {
		var c ChatSummary
		var chatID int64
		if err := rows.Scan(&chatID, &c.GUID, &c.Title, &c.Kind, &c.ChatIdentifier, &c.RoomName, &c.Service, &c.ParticipantCount, &c.MessageCount, &c.LatestMessageDate); err != nil {
			return nil, err
		}
		c.ChatID = strconv.FormatInt(chatID, 10)
		out = append(out, c)
	}
	return out, rows.Err()
}

func populateParticipantHandles(ctx context.Context, db *sql.DB, chats []ChatSummary) error {
	for i := range chats {
		handles, err := participantHandles(ctx, db, chats[i].ChatID)
		if err != nil {
			return err
		}
		chats[i].ParticipantHandles = handles
	}
	return nil
}

func participantHandles(ctx context.Context, db *sql.DB, chatID string) ([]string, error) {
	id, err := parseID(chatID, "chat")
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `
select h.handle
from chat_participants cp
join handles h on h.source_rowid = cp.handle_rowid
where cp.chat_rowid = ?
  and nullif(trim(h.handle), '') is not null
order by h.handle
limit 6
`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var handle string
		if err := rows.Scan(&handle); err != nil {
			return nil, err
		}
		out = append(out, handle)
	}
	return out, rows.Err()
}
