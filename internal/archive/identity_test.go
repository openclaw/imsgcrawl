package archive

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openclaw/imsgcrawl/internal/messages"
)

func TestMergeRejectsSourceIdentityRemapping(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		mutate func(*messages.ArchiveData)
	}{
		{name: "message rowid reused", mutate: func(data *messages.ArchiveData) { data.Messages[0].GUID = "different-message" }},
		{name: "message guid moved", mutate: func(data *messages.ArchiveData) { data.Messages[0].SourceRowID = 99 }},
		{name: "chat rowid reused", mutate: func(data *messages.ArchiveData) { data.Chats[0].GUID = "different-chat" }},
		{name: "handle rowid reused", mutate: func(data *messages.ArchiveData) { data.Handles[0].ID = "+15559999" }},
		{name: "handle identity moved", mutate: func(data *messages.ArchiveData) { data.Handles[0].SourceRowID = 99 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			st := openTestArchive(t, ctx)
			defer func() { _ = st.Close() }()
			if err := st.Import(ctx, fixtureArchiveData(), now, false); err != nil {
				t.Fatal(err)
			}
			incoming := fixtureArchiveData()
			test.mutate(&incoming)
			before := archiveContents(t, st)
			err := st.Import(ctx, incoming, now.Add(time.Minute), false)
			if err == nil || !strings.Contains(err.Error(), "sync --restore") {
				t.Fatalf("merge error = %v", err)
			}
			assertArchiveUnchanged(t, st, before)
		})
	}
}

func TestRestoreAllowsSourceIdentityRemapping(t *testing.T) {
	ctx := context.Background()
	st := openTestArchive(t, ctx)
	defer func() { _ = st.Close() }()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	if err := st.Import(ctx, fixtureArchiveData(), now, false); err != nil {
		t.Fatal(err)
	}
	if _, err := st.store.DB().Exec(`delete from sync_state where key='source_identity'`); err != nil {
		t.Fatal(err)
	}
	equivalent := fixtureArchiveData()
	equivalent.SourcePath = "/private/test/../test/chat.db"
	if err := st.Import(ctx, equivalent, now.Add(30*time.Second), false); err != nil {
		t.Fatalf("normalized source path rejected: %v", err)
	}
	incoming := fixtureArchiveData()
	incoming.Messages[0].GUID = "replacement-message"
	if err := st.Import(ctx, incoming, now.Add(time.Minute), true); err != nil {
		t.Fatal(err)
	}
	if got := scalar(t, st.store.DB(), `select count(*) from messages where guid = 'message-one'`); got != 0 {
		t.Fatalf("restore retained prior identity: %d", got)
	}
	if got := scalar(t, st.store.DB(), `select count(*) from messages where guid = 'replacement-message'`); got != 1 {
		t.Fatalf("restore replacement identity = %d", got)
	}
}

func TestMergeAdoptsLegacyRelativeSourcePathOnce(t *testing.T) {
	ctx := context.Background()
	st := openTestArchive(t, ctx)
	defer func() { _ = st.Close() }()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	data := fixtureArchiveData()
	if err := st.Import(ctx, data, now, false); err != nil {
		t.Fatal(err)
	}
	if _, err := st.store.DB().Exec(`delete from sync_state where key='source_identity';
update sync_state set value='relative/chat.db' where key='source_path'`); err != nil {
		t.Fatal(err)
	}
	unrelated := data
	unrelated.SourcePath = "/other/database.db"
	before := archiveContents(t, st)
	err := st.Import(ctx, unrelated, now.Add(30*time.Second), false)
	if err == nil || !strings.Contains(err.Error(), "sync --restore") {
		t.Fatalf("unrelated source was adopted: %v", err)
	}
	assertArchiveUnchanged(t, st, before)
	data.SourcePath = "/new-working-directory/relative/chat.db"
	if err := st.Import(ctx, data, now.Add(time.Minute), false); err != nil {
		t.Fatalf("legacy relative path was not adopted: %v", err)
	}
	data.SourcePath = "/another-source/chat.db"
	before = archiveContents(t, st)
	err = st.Import(ctx, data, now.Add(2*time.Minute), false)
	if err == nil || !strings.Contains(err.Error(), "sync --restore") {
		t.Fatalf("adopted source identity was not enforced: %v", err)
	}
	assertArchiveUnchanged(t, st, before)
}

func TestMergeRejectsDifferentSourceDatabase(t *testing.T) {
	ctx := context.Background()
	st := openTestArchive(t, ctx)
	defer func() { _ = st.Close() }()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	if err := st.Import(ctx, fixtureArchiveData(), now, false); err != nil {
		t.Fatal(err)
	}
	incoming := fixtureArchiveData()
	incoming.SourcePath = "/private/other/chat.db"
	before := archiveContents(t, st)
	err := st.Import(ctx, incoming, now.Add(time.Minute), false)
	if err == nil || !strings.Contains(err.Error(), "sync --restore") {
		t.Fatalf("source lineage error = %v", err)
	}
	assertArchiveUnchanged(t, st, before)
	if err := st.Import(ctx, incoming, now.Add(2*time.Minute), true); err != nil {
		t.Fatal(err)
	}
	var sourcePath string
	if err := st.store.DB().QueryRow(`select value from sync_state where key='source_path'`).Scan(&sourcePath); err != nil {
		t.Fatal(err)
	}
	if sourcePath != incoming.SourcePath {
		t.Fatalf("restored source path = %q", sourcePath)
	}
}

func TestRestoreRejectsDuplicateIncomingGUIDsWithoutChangingArchive(t *testing.T) {
	ctx := context.Background()
	st := openTestArchive(t, ctx)
	defer func() { _ = st.Close() }()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	if err := st.Import(ctx, fixtureArchiveData(), now, false); err != nil {
		t.Fatal(err)
	}
	incoming := fixtureArchiveData()
	incoming.Messages[1].GUID = incoming.Messages[0].GUID
	before := archiveContents(t, st)
	err := st.Import(ctx, incoming, now.Add(time.Minute), true)
	if err == nil || !strings.Contains(err.Error(), "appears at rowids") {
		t.Fatalf("duplicate restore error = %v", err)
	}
	assertArchiveUnchanged(t, st, before)
}

func TestMergeRejectsExistingDuplicateIdentity(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		insert string
	}{
		{name: "message", insert: `insert into messages(source_rowid, guid) values(99, 'message-one')`},
		{name: "chat", insert: `insert into chats(source_rowid, guid) values(99, 'chat-one')`},
		{name: "handle", insert: `insert into handles(source_rowid, handle, service)
select 99, handle, service from handles where source_rowid = 1`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			st := openTestArchive(t, ctx)
			defer func() { _ = st.Close() }()
			data := fixtureArchiveData()
			if err := st.Import(ctx, data, now, false); err != nil {
				t.Fatal(err)
			}
			if _, err := st.store.DB().Exec(test.insert); err != nil {
				t.Fatal(err)
			}
			before := archiveContents(t, st)
			err := st.Import(ctx, data, now.Add(time.Minute), false)
			if err == nil || !strings.Contains(err.Error(), "sync --restore") {
				t.Fatalf("duplicate %s merge error = %v", test.name, err)
			}
			assertArchiveUnchanged(t, st, before)
		})
	}
}

func TestSyntheticTombstoneCollisionAllocatesAnotherRow(t *testing.T) {
	ctx := context.Background()
	st := openTestArchive(t, ctx)
	defer func() { _ = st.Close() }()
	collidingRowID := syntheticRowID("message", "target-guid", 0)
	if _, err := st.store.DB().Exec(`insert into messages(source_rowid, guid, deleted_at, deletion_reason)
values(?, 'other-guid', '2026-07-18T12:00:00Z', 'test')`, collidingRowID); err != nil {
		t.Fatal(err)
	}
	data := messages.ArchiveData{DeletedMessages: []string{"target-guid"}}
	if err := st.Import(ctx, data, time.Date(2026, 7, 18, 12, 1, 0, 0, time.UTC), false); err != nil {
		t.Fatal(err)
	}
	assertTombstone(t, st.store.DB(), "messages", "guid", "target-guid", deletionExplicitFeed)
	if got := scalar(t, st.store.DB(), `select count(*) from messages where source_rowid < 0`); got != 2 {
		t.Fatalf("synthetic tombstone rows = %d", got)
	}
}

func TestBlankGUIDEventsUseRowIDFallback(t *testing.T) {
	ctx := context.Background()
	st := openTestArchive(t, ctx)
	defer func() { _ = st.Close() }()
	data := messages.ArchiveData{Messages: []messages.Message{
		{SourceRowID: 1, GUID: "", Text: "same", FullyUnsent: true},
		{SourceRowID: 2, GUID: "", Text: "same"},
	}}
	if err := st.Import(ctx, data, time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC), true); err != nil {
		t.Fatal(err)
	}
	if got := scalar(t, st.store.DB(), `select count(*) from message_events`); got != 2 {
		t.Fatalf("blank-GUID events = %d", got)
	}
	assertTombstone(t, st.store.DB(), "messages", "source_rowid", int64(1), deletionSourceUnsent)
	assertActive(t, st.store.DB(), "messages", "source_rowid", int64(2))
}

// Compare logical rows: SQLite/WAL file bytes can change without data changes.
func archiveContents(t *testing.T, st *Store) string {
	t.Helper()
	var contents strings.Builder
	for _, table := range []string{
		"handles", "chats", "chat_participants", "chat_messages",
		"messages", "message_events", "messages_fts", "sync_state",
	} {
		rows, err := st.store.DB().Query("select * from " + table + " order by 1, 2")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		columns, err := rows.Columns()
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintln(&contents, table, columns)
		values := make([]any, len(columns))
		targets := make([]any, len(columns))
		for i := range values {
			targets[i] = &values[i]
		}
		for rows.Next() {
			if err := rows.Scan(targets...); err != nil {
				t.Fatal(err)
			}
			fmt.Fprintf(&contents, "%#v\n", values)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
	}
	return contents.String()
}

func assertArchiveUnchanged(t *testing.T, st *Store, before string) {
	t.Helper()
	if after := archiveContents(t, st); after != before {
		t.Fatalf("rejected import changed archive:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}
