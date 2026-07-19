package archive

import (
	"context"
	"path/filepath"
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
			st, err := Open(ctx, filepath.Join(t.TempDir(), "archive.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = st.Close() }()
			if err := st.Import(ctx, fixtureArchiveData(), now, false); err != nil {
				t.Fatal(err)
			}
			incoming := fixtureArchiveData()
			test.mutate(&incoming)
			err = st.Import(ctx, incoming, now.Add(time.Minute), false)
			if err == nil || !strings.Contains(err.Error(), "sync --restore") {
				t.Fatalf("merge error = %v", err)
			}
			if got := scalar(t, st.store.DB(), `select count(*) from messages where guid = 'message-one'`); got != 1 {
				t.Fatalf("failed merge changed archive: %d", got)
			}
		})
	}
}

func TestRestoreAllowsSourceIdentityRemapping(t *testing.T) {
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
	st, err := Open(ctx, filepath.Join(t.TempDir(), "archive.db"))
	if err != nil {
		t.Fatal(err)
	}
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
	err = st.Import(ctx, unrelated, now.Add(30*time.Second), false)
	if err == nil || !strings.Contains(err.Error(), "sync --restore") {
		t.Fatalf("unrelated source was adopted: %v", err)
	}
	data.SourcePath = "/new-working-directory/relative/chat.db"
	if err := st.Import(ctx, data, now.Add(time.Minute), false); err != nil {
		t.Fatalf("legacy relative path was not adopted: %v", err)
	}
	data.SourcePath = "/another-source/chat.db"
	err = st.Import(ctx, data, now.Add(2*time.Minute), false)
	if err == nil || !strings.Contains(err.Error(), "sync --restore") {
		t.Fatalf("adopted source identity was not enforced: %v", err)
	}
}

func TestMergeRejectsExistingDuplicateHandleIdentity(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "archive.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	data := fixtureArchiveData()
	if err := st.Import(ctx, data, now, false); err != nil {
		t.Fatal(err)
	}
	if _, err := st.store.DB().Exec(`insert into handles(source_rowid, handle, service) values(99, ?, ?)`, data.Handles[0].ID, data.Handles[0].Service); err != nil {
		t.Fatal(err)
	}
	err = st.Import(ctx, data, now.Add(time.Minute), false)
	if err == nil || !strings.Contains(err.Error(), "sync --restore") {
		t.Fatalf("duplicate handle merge error = %v", err)
	}
}

func TestMergeRejectsDifferentSourceDatabase(t *testing.T) {
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
	incoming := fixtureArchiveData()
	incoming.SourcePath = "/private/other/chat.db"
	err = st.Import(ctx, incoming, now.Add(time.Minute), false)
	if err == nil || !strings.Contains(err.Error(), "sync --restore") {
		t.Fatalf("source lineage error = %v", err)
	}
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
	st, err := Open(ctx, filepath.Join(t.TempDir(), "archive.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	if err := st.Import(ctx, fixtureArchiveData(), now, false); err != nil {
		t.Fatal(err)
	}
	incoming := fixtureArchiveData()
	incoming.Messages[1].GUID = incoming.Messages[0].GUID
	err = st.Import(ctx, incoming, now.Add(time.Minute), true)
	if err == nil || !strings.Contains(err.Error(), "appears at rowids") {
		t.Fatalf("duplicate restore error = %v", err)
	}
	if got := scalar(t, st.store.DB(), `select count(*) from messages`); got != 2 {
		t.Fatalf("failed restore changed archive: %d", got)
	}
}

func TestMergeRejectsExistingDuplicateGUID(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		insert string
	}{
		{name: "message", insert: `insert into messages(source_rowid, guid) values(99, 'message-one')`},
		{name: "chat", insert: `insert into chats(source_rowid, guid) values(99, 'chat-one')`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			st, err := Open(ctx, filepath.Join(t.TempDir(), "archive.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = st.Close() }()
			data := fixtureArchiveData()
			if err := st.Import(ctx, data, now, false); err != nil {
				t.Fatal(err)
			}
			if _, err := st.store.DB().Exec(test.insert); err != nil {
				t.Fatal(err)
			}
			err = st.Import(ctx, data, now.Add(time.Minute), false)
			if err == nil || !strings.Contains(err.Error(), "sync --restore") {
				t.Fatalf("duplicate %s merge error = %v", test.name, err)
			}
		})
	}
}

func TestSyntheticTombstoneCollisionAllocatesAnotherRow(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "archive.db"))
	if err != nil {
		t.Fatal(err)
	}
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
	st, err := Open(ctx, filepath.Join(t.TempDir(), "archive.db"))
	if err != nil {
		t.Fatal(err)
	}
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
