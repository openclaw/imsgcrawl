package archive

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/openclaw/imsgcrawl/internal/messages"
	_ "modernc.org/sqlite"
)

func TestImportMergeRestoreTombstonesAndRevisions(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "archive.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	base := fixtureArchiveData()
	if err := st.Import(ctx, base, now, false); err != nil {
		t.Fatal(err)
	}
	if got := scalar(t, st.store.DB(), `select count(*) from message_events`); got != 2 {
		t.Fatalf("initial events = %d", got)
	}

	partial := fixtureArchiveData()
	partial.Handles = partial.Handles[:1]
	partial.Chats = partial.Chats[:1]
	partial.Participants = partial.Participants[:1]
	partial.ChatMessages = partial.ChatMessages[:1]
	partial.Messages = partial.Messages[:1]
	partial.Messages[0].Text = "edited text"
	partial.Messages[0].DateEdited = 804_340_800_000_000_000
	partial.Messages[0].RevisionData = []byte("edit-history")
	if err := st.Import(ctx, partial, now.Add(time.Minute), false); err != nil {
		t.Fatal(err)
	}
	assertCount(t, st.store.DB(), "messages", 2)
	if got := scalar(t, st.store.DB(), `select count(*) from message_events`); got != 3 {
		t.Fatalf("edited events = %d", got)
	}
	if got := scalar(t, st.store.DB(), `select count(*) from message_events where event_type = 'message_edited'`); got != 1 {
		t.Fatalf("edited event count = %d", got)
	}
	if err := st.Import(ctx, partial, now.Add(2*time.Minute), false); err != nil {
		t.Fatal(err)
	}
	if got := scalar(t, st.store.DB(), `select count(*) from message_events`); got != 3 {
		t.Fatalf("repeat import duplicated event: %d", got)
	}

	partial.DeletedMessages = []string{"message-one", "message-two", "deleted-before-first-sync"}
	partial.DeletedChats = []string{"chat-one", "chat-two", "deleted-chat-before-first-sync"}
	if err := st.Import(ctx, partial, now.Add(3*time.Minute), false); err != nil {
		t.Fatal(err)
	}
	assertActive(t, st.store.DB(), "messages", "guid", "message-one")
	assertActive(t, st.store.DB(), "chats", "guid", "chat-one")
	assertTombstone(t, st.store.DB(), "messages", "guid", "message-two", deletionExplicitFeed)
	assertTombstone(t, st.store.DB(), "chat_messages", "message_rowid", int64(2), "parent-message-"+deletionExplicitFeed)
	assertTombstone(t, st.store.DB(), "messages", "guid", "deleted-before-first-sync", deletionExplicitFeed)
	assertTombstone(t, st.store.DB(), "chats", "guid", "chat-two", deletionExplicitFeed)
	assertTombstone(t, st.store.DB(), "chat_participants", "chat_rowid", int64(2), "parent-chat-"+deletionExplicitFeed)
	assertTombstone(t, st.store.DB(), "chats", "guid", "deleted-chat-before-first-sync", deletionExplicitFeed)
	if got := scalar(t, st.store.DB(), `select count(*) from messages_fts where messages_fts match 'destination'`); got != 0 {
		t.Fatalf("tombstoned message remained searchable: %d", got)
	}
	if err := st.Import(ctx, fixtureArchiveData(), now.Add(4*time.Minute), false); err != nil {
		t.Fatal(err)
	}
	assertTombstone(t, st.store.DB(), "messages", "guid", "message-two", deletionExplicitFeed)
	late := fixtureArchiveData()
	late.Chats = append(late.Chats, messages.Chat{SourceRowID: 3, GUID: "deleted-chat-before-first-sync"})
	late.Participants = append(late.Participants, messages.Participant{ChatRowID: 3, HandleRowID: 1})
	late.Messages = append(late.Messages, messages.Message{SourceRowID: 3, GUID: "deleted-before-first-sync", Date: 30, Text: "must stay deleted"})
	late.ChatMessages = append(late.ChatMessages, messages.ChatMessage{ChatRowID: 3, MessageRowID: 3})
	if err := st.Import(ctx, late, now.Add(4*time.Minute+30*time.Second), false); err != nil {
		t.Fatal(err)
	}
	assertTombstone(t, st.store.DB(), "messages", "guid", "deleted-before-first-sync", deletionExplicitFeed)
	assertTombstone(t, st.store.DB(), "chats", "guid", "deleted-chat-before-first-sync", deletionExplicitFeed)
	assertTombstone(t, st.store.DB(), "chat_participants", "chat_rowid", int64(3), "parent-chat-"+deletionExplicitFeed)
	assertTombstone(t, st.store.DB(), "chat_messages", "message_rowid", int64(3), "parent-message-"+deletionExplicitFeed)
	if got := scalar(t, st.store.DB(), `select count(*) from messages where guid = 'deleted-before-first-sync'`); got != 1 {
		t.Fatalf("resolved message tombstone rows = %d", got)
	}
	if got := scalar(t, st.store.DB(), `select count(*) from chats where guid = 'deleted-chat-before-first-sync'`); got != 1 {
		t.Fatalf("resolved chat tombstone rows = %d", got)
	}

	restoreData := partial
	restoreData.DeletedMessages = nil
	restoreData.DeletedChats = nil
	if err := st.Import(ctx, restoreData, now.Add(5*time.Minute), true); err != nil {
		t.Fatal(err)
	}
	assertCount(t, st.store.DB(), "messages", 1)
	var deletedAt sql.NullString
	if err := st.store.DB().QueryRowContext(ctx, `select deleted_at from messages where guid = 'message-one'`).Scan(&deletedAt); err != nil {
		t.Fatal(err)
	}
	if deletedAt.Valid {
		t.Fatalf("restore retained old tombstone: %q", deletedAt.String)
	}
	if got := scalar(t, st.store.DB(), `select count(*) from message_events`); got != 1 {
		t.Fatalf("restore event history = %d", got)
	}

	unsent := restoreData
	unsent.Messages = append([]messages.Message(nil), restoreData.Messages...)
	unsent.Messages[0].DateRetracted = 0
	unsent.Messages[0].HasUnsentParts = true
	unsent.Messages[0].FullyUnsent = true
	if err := st.Import(ctx, unsent, now.Add(6*time.Minute), false); err != nil {
		t.Fatal(err)
	}
	assertTombstone(t, st.store.DB(), "messages", "guid", "message-one", deletionSourceUnsent)
	if got := scalar(t, st.store.DB(), `select count(*) from message_events where event_type = 'message_unsent'`); got != 1 {
		t.Fatalf("unsent event count = %d", got)
	}
}

func TestPartialUnsendRemainsActiveAndSummaryMetadataIsNotARevision(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "archive.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	data := fixtureArchiveData()
	data.Messages = data.Messages[:1]
	data.ChatMessages = data.ChatMessages[:1]
	data.Messages[0].RevisionData = []byte("unrelated-summary-v1")
	if err := st.Import(ctx, data, now, false); err != nil {
		t.Fatal(err)
	}
	data.Messages[0].RevisionData = []byte("unrelated-summary-v2")
	if err := st.Import(ctx, data, now.Add(time.Minute), false); err != nil {
		t.Fatal(err)
	}
	if got := scalar(t, st.store.DB(), `select count(*) from message_events`); got != 1 {
		t.Fatalf("unrelated summary metadata events = %d", got)
	}
	var revisionData []byte
	if err := st.store.DB().QueryRow(`select revision_data from message_events`).Scan(&revisionData); err != nil {
		t.Fatal(err)
	}
	if len(revisionData) != 0 {
		t.Fatalf("ordinary event retained unrelated revision data: %x", revisionData)
	}
	data.Messages[0].HasUnsentParts = true
	data.Messages[0].RevisionData = []byte("partial-unsend")
	if err := st.Import(ctx, data, now.Add(2*time.Minute), false); err != nil {
		t.Fatal(err)
	}
	var deletedAt sql.NullString
	if err := st.store.DB().QueryRow(`select deleted_at from messages where guid = 'message-one'`).Scan(&deletedAt); err != nil {
		t.Fatal(err)
	}
	if deletedAt.Valid {
		t.Fatalf("partial unsend tombstoned message: %q", deletedAt.String)
	}
	if got := scalar(t, st.store.DB(), `select count(*) from message_events where event_type = 'message_partial_unsent'`); got != 1 {
		t.Fatalf("partial unsend events = %d", got)
	}
}

func TestOpenMigratesVersionOneArchive(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "v1.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
create table messages (
  source_rowid integer primary key, guid text not null, handle_rowid integer not null default 0,
  date integer not null default 0, service text, is_from_me integer not null default 0,
  text text, has_attachments integer not null default 0
);
create table schema_migrations(version integer not null);
insert into schema_migrations(version) values(1);
insert into messages(source_rowid, guid, date, text) values(7, 'legacy-guid', 42, 'legacy text');`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	st, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	version, err := st.store.SchemaVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if version != schemaVersion {
		t.Fatalf("schema version = %d", version)
	}
	for _, column := range []string{"date_edited", "date_retracted", "revision_data", "deleted_at", "deletion_reason"} {
		var exists int
		if err := st.store.DB().QueryRowContext(ctx, `select count(*) from pragma_table_info('messages') where name = ?`, column).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if exists != 1 {
			t.Fatalf("missing migrated messages.%s", column)
		}
	}
	for _, table := range []string{"handles", "chats", "chat_participants", "chat_messages", "message_events"} {
		for _, column := range []string{"deleted_at", "deletion_reason"} {
			var exists int
			if err := st.store.DB().QueryRowContext(ctx, `select count(*) from pragma_table_info(?) where name = ?`, table, column).Scan(&exists); err != nil {
				t.Fatal(err)
			}
			if exists != 1 {
				t.Fatalf("missing migrated %s.%s", table, column)
			}
		}
	}
	if got := scalar(t, st.store.DB(), `select count(*) from message_events where message_guid = 'legacy-guid'`); got != 1 {
		t.Fatalf("seeded legacy events = %d", got)
	}
}

func TestDeletedChatMessagesAreNotSearchable(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "archive.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	if err := st.Import(ctx, fixtureArchiveData(), now, false); err != nil {
		t.Fatal(err)
	}
	if err := st.Import(ctx, messages.ArchiveData{DeletedChats: []string{"chat-one"}}, now.Add(time.Minute), false); err != nil {
		t.Fatal(err)
	}
	rows, err := st.Search(ctx, "original", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("deleted chat search rows = %#v", rows)
	}
	count, err := st.CountSearch(ctx, "original")
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("deleted chat search count = %d", count)
	}
}

func TestOrphanMessagesRemainSearchable(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "archive.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	data := fixtureArchiveData()
	data.ChatMessages = data.ChatMessages[:1]
	if err := st.Import(ctx, data, time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC), false); err != nil {
		t.Fatal(err)
	}
	rows, err := st.Search(ctx, "destination", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].GUID != "message-two" || rows[0].ChatID != "" {
		t.Fatalf("orphan search rows = %#v", rows)
	}
	count, err := st.CountSearch(ctx, "destination")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("orphan search count = %d", count)
	}
}

func fixtureArchiveData() messages.ArchiveData {
	return messages.ArchiveData{
		SourcePath: "/private/test/chat.db", SourceBytes: 123,
		SourceModifiedAt: time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC),
		ExtractedAt:      time.Date(2026, 7, 18, 10, 1, 0, 0, time.UTC),
		Handles:          []messages.Handle{{SourceRowID: 1, ID: "+15550001", Service: "iMessage"}, {SourceRowID: 2, ID: "+15550002", Service: "iMessage"}},
		Chats:            []messages.Chat{{SourceRowID: 1, GUID: "chat-one"}, {SourceRowID: 2, GUID: "chat-two"}},
		Participants:     []messages.Participant{{ChatRowID: 1, HandleRowID: 1}, {ChatRowID: 2, HandleRowID: 2}},
		ChatMessages:     []messages.ChatMessage{{ChatRowID: 1, MessageRowID: 1}, {ChatRowID: 2, MessageRowID: 2}},
		Messages: []messages.Message{
			{SourceRowID: 1, GUID: "message-one", HandleRowID: 1, Date: 10, Text: "original text", DateEditedAvailable: true, DateRetractedAvailable: true, RevisionDataAvailable: true},
			{SourceRowID: 2, GUID: "message-two", HandleRowID: 2, Date: 20, Text: "destination only", DateEditedAvailable: true, DateRetractedAvailable: true, RevisionDataAvailable: true},
		},
	}
}

func scalar(t *testing.T, db *sql.DB, query string) int64 {
	t.Helper()
	var got int64
	if err := db.QueryRow(query).Scan(&got); err != nil {
		t.Fatal(err)
	}
	return got
}

func assertCount(t *testing.T, db *sql.DB, table string, want int64) {
	t.Helper()
	if got := scalar(t, db, "select count(*) from "+table); got != want {
		t.Fatalf("%s count = %d, want %d", table, got, want)
	}
}

func assertTombstone(t *testing.T, db *sql.DB, table, key string, value any, reason string) {
	t.Helper()
	var deletedAt, gotReason sql.NullString
	if err := db.QueryRow(`select deleted_at, deletion_reason from `+table+` where `+key+` = ?`, value).Scan(&deletedAt, &gotReason); err != nil {
		t.Fatal(err)
	}
	if !deletedAt.Valid || gotReason.String != reason {
		t.Fatalf("%s tombstone = deleted_at:%v reason:%q", table, deletedAt, gotReason.String)
	}
}

func assertActive(t *testing.T, db *sql.DB, table, key string, value any) {
	t.Helper()
	var deletedAt sql.NullString
	if err := db.QueryRow(`select deleted_at from `+table+` where `+key+` = ?`, value).Scan(&deletedAt); err != nil {
		t.Fatal(err)
	}
	if deletedAt.Valid {
		t.Fatalf("%s.%s %v unexpectedly tombstoned at %q", table, key, value, deletedAt.String)
	}
}
