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
	rows, err := db.QueryContext(ctx, participantHandlesSQL, id)
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
