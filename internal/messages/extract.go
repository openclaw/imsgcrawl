package messages

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"time"
)

type ArchiveData struct {
	SourcePath       string
	SourceBytes      int64
	SourceModifiedAt time.Time
	ExtractedAt      time.Time
	Handles          []Handle
	Chats            []Chat
	Participants     []Participant
	ChatMessages     []ChatMessage
	Messages         []Message
}

type Handle struct {
	SourceRowID       int64
	ID                string
	Service           string
	UncanonicalizedID string
}

type Chat struct {
	SourceRowID    int64
	GUID           string
	ChatIdentifier string
	ServiceName    string
	DisplayName    string
	RoomName       string
	IsArchived     bool
}

type Participant struct {
	ChatRowID   int64
	HandleRowID int64
}

type ChatMessage struct {
	ChatRowID    int64
	MessageRowID int64
}

type Message struct {
	SourceRowID    int64
	GUID           string
	HandleRowID    int64
	Date           int64
	Service        string
	IsFromMe       bool
	Text           string
	HasAttachments bool
}

func ExtractArchive(ctx context.Context, path string) (ArchiveData, error) {
	snap, err := SnapshotPath(path)
	if err != nil {
		return ArchiveData{}, err
	}
	defer func() { _ = snap.Close() }()
	st, err := openSnapshot(ctx, snap.Path)
	if err != nil {
		return ArchiveData{}, err
	}
	defer func() { _ = st.Close() }()
	if err := requireArchiveTables(ctx, st.DB()); err != nil {
		return ArchiveData{}, err
	}
	info, err := os.Stat(snap.SourcePath)
	if err != nil {
		return ArchiveData{}, err
	}
	data := ArchiveData{
		SourcePath:       snap.SourcePath,
		SourceBytes:      info.Size(),
		SourceModifiedAt: info.ModTime().UTC(),
		ExtractedAt:      time.Now().UTC(),
	}
	if data.Handles, err = extractHandles(ctx, st.DB()); err != nil {
		return ArchiveData{}, err
	}
	if data.Chats, err = extractChats(ctx, st.DB()); err != nil {
		return ArchiveData{}, err
	}
	if data.Participants, err = extractParticipants(ctx, st.DB()); err != nil {
		return ArchiveData{}, err
	}
	if data.ChatMessages, err = extractChatMessages(ctx, st.DB()); err != nil {
		return ArchiveData{}, err
	}
	if data.Messages, err = extractMessages(ctx, st.DB()); err != nil {
		return ArchiveData{}, err
	}
	return data, nil
}

func requireArchiveTables(ctx context.Context, db *sql.DB) error {
	for _, table := range []string{"chat_message_join", "message_attachment_join"} {
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

func extractHandles(ctx context.Context, db *sql.DB) ([]Handle, error) {
	rows, err := db.QueryContext(ctx, extractHandlesSQL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Handle
	for rows.Next() {
		var h Handle
		if err := rows.Scan(&h.SourceRowID, &h.ID, &h.Service, &h.UncanonicalizedID); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func extractChats(ctx context.Context, db *sql.DB) ([]Chat, error) {
	rows, err := db.QueryContext(ctx, extractChatsSQL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Chat
	for rows.Next() {
		var c Chat
		var archived int
		if err := rows.Scan(&c.SourceRowID, &c.GUID, &c.ChatIdentifier, &c.ServiceName, &c.DisplayName, &c.RoomName, &archived); err != nil {
			return nil, err
		}
		c.IsArchived = archived != 0
		out = append(out, c)
	}
	return out, rows.Err()
}

func extractParticipants(ctx context.Context, db *sql.DB) ([]Participant, error) {
	rows, err := db.QueryContext(ctx, extractParticipantsSQL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Participant
	for rows.Next() {
		var p Participant
		if err := rows.Scan(&p.ChatRowID, &p.HandleRowID); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func extractChatMessages(ctx context.Context, db *sql.DB) ([]ChatMessage, error) {
	rows, err := db.QueryContext(ctx, extractChatMessagesSQL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []ChatMessage
	for rows.Next() {
		var cm ChatMessage
		if err := rows.Scan(&cm.ChatRowID, &cm.MessageRowID); err != nil {
			return nil, err
		}
		out = append(out, cm)
	}
	return out, rows.Err()
}

func extractMessages(ctx context.Context, db *sql.DB) ([]Message, error) {
	rows, err := db.QueryContext(ctx, extractMessagesSQL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Message
	for rows.Next() {
		var m Message
		var fromMe int
		var hasAttachments int
		var attributedBody []byte
		if err := rows.Scan(&m.SourceRowID, &m.GUID, &m.HandleRowID, &m.Date, &m.Service, &fromMe, &m.Text, &attributedBody, &hasAttachments); err != nil {
			return nil, err
		}
		if m.Text == "" {
			m.Text = decodeAttributedBody(attributedBody)
		}
		m.IsFromMe = fromMe != 0
		m.HasAttachments = hasAttachments != 0
		out = append(out, m)
	}
	return out, rows.Err()
}
