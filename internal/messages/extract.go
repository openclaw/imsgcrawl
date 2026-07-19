package messages

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
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
	DeletedChats     []string
	DeletedMessages  []string
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
	SourceRowID            int64
	GUID                   string
	HandleRowID            int64
	Date                   int64
	Service                string
	IsFromMe               bool
	Text                   string
	TextAvailable          bool
	TextIsCurrent          bool
	HasAttachments         bool
	DateEdited             int64
	DateRetracted          int64
	RevisionData           []byte
	HasEdits               bool
	HasUnsentParts         bool
	FullyUnsent            bool
	RevisionAt             int64
	RevisionIdentity       string
	DateEditedAvailable    bool
	DateRetractedAvailable bool
	RevisionDataAvailable  bool
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
	if data.DeletedChats, err = extractDeletedGUIDs(ctx, st.DB(), "sync_deleted_chats"); err != nil {
		return ArchiveData{}, err
	}
	if data.DeletedMessages, err = extractDeletedGUIDs(ctx, st.DB(), "sync_deleted_messages"); err != nil {
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
	query, availability, err := revisionAwareMessagesQuery(ctx, db)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, query)
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
		if err := rows.Scan(&m.SourceRowID, &m.GUID, &m.HandleRowID, &m.Date, &m.Service, &fromMe, &m.Text, &attributedBody, &hasAttachments, &m.DateEdited, &m.DateRetracted, &m.RevisionData); err != nil {
			return nil, err
		}
		if m.Text == "" {
			if text, ok := decodeAttributedBodyValue(attributedBody); ok {
				m.Text = text
				m.TextIsCurrent = true
			}
		}
		m.TextAvailable = true
		m.IsFromMe = fromMe != 0
		m.HasAttachments = hasAttachments != 0
		m.DateEditedAvailable = availability.DateEdited
		m.DateRetractedAvailable = availability.DateRetracted
		m.RevisionDataAvailable = availability.RevisionData
		m.ApplyRevisionData()
		out = append(out, m)
	}
	return out, rows.Err()
}

type revisionAvailability struct {
	DateEdited    bool
	DateRetracted bool
	RevisionData  bool
}

func revisionAwareMessagesQuery(ctx context.Context, db *sql.DB) (string, revisionAvailability, error) {
	columns, err := tableColumns(ctx, db, "message")
	if err != nil {
		return "", revisionAvailability{}, err
	}
	availability := revisionAvailability{
		DateEdited: columns["date_edited"], DateRetracted: columns["date_retracted"],
		RevisionData: columns["message_summary_info"],
	}
	query := extractMessagesSQL
	for placeholder, column := range map[string]string{
		"{{DATE_EDITED}}":          "date_edited",
		"{{DATE_RETRACTED}}":       "date_retracted",
		"{{MESSAGE_SUMMARY_INFO}}": "message_summary_info",
	} {
		expression := "0"
		if column == "message_summary_info" {
			expression = "x''"
		}
		if columns[column] {
			expression = "coalesce(m." + column + ", " + expression + ")"
		}
		query = strings.ReplaceAll(query, placeholder, expression)
	}
	return query, availability, nil
}

func (m *Message) ApplyRevisionData() {
	// date_edited confirms that the source fallback may no longer be current.
	// Only expose it again after reconstructing the latest revision.
	if m.DateEdited > 0 {
		m.TextAvailable = false
	}
	root, ok := messageSummaryRoot(m.RevisionData)
	if !ok {
		return
	}
	revision := parseMessageSummaryRoot(root)
	m.HasEdits = revision.HasEdits
	m.HasUnsentParts = revision.HasUnsentParts
	m.FullyUnsent = revision.FullyUnsent
	m.RevisionAt = revision.RevisionAt
	m.RevisionIdentity = revision.Identity
	if m.HasEdits || m.HasUnsentParts {
		if m.TextIsCurrent {
			m.TextAvailable = true
			return
		}
		text, available := reconstructCurrentText(root, m.Text)
		m.TextAvailable = available
		if available {
			m.Text = text
		}
	}
}

func tableColumns(ctx context.Context, db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, "select name from pragma_table_info(?)", table)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	columns := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		columns[name] = true
	}
	return columns, rows.Err()
}

func extractDeletedGUIDs(ctx context.Context, db *sql.DB, table string) ([]string, error) {
	var exists int
	if err := db.QueryRowContext(ctx, `select count(*) from sqlite_master where type = 'table' and name = ?`, table).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, nil
	}
	columns, err := tableColumns(ctx, db, table)
	if err != nil {
		return nil, err
	}
	if !columns["guid"] {
		return nil, nil
	}
	rows, err := db.QueryContext(ctx, "select distinct trim(guid) as guid from "+table+" where nullif(trim(guid), '') is not null order by guid")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var guid string
		if err := rows.Scan(&guid); err != nil {
			return nil, err
		}
		out = append(out, guid)
	}
	return out, rows.Err()
}
