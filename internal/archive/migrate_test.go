package archive

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
)

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
insert into messages(source_rowid, guid, date, text) values(7, 'legacy-guid', 42, 'legacy text');
insert into messages(source_rowid, guid, handle_rowid, date, service, is_from_me, text, has_attachments)
values(8, 'populated-legacy-guid', 19, 84, 'iMessage', 1, 'populated legacy text', 1);`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	st, err := OpenExisting(ctx, path)
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
	assertCount(t, st.store.DB(), "messages", 2)
	assertCount(t, st.store.DB(), "message_events", 2)
	for _, want := range []struct {
		rowID   int64
		payload messageEventPayload
	}{
		{7, messageEventPayload{GUID: "legacy-guid", Date: 42, Text: "legacy text"}},
		{8, messageEventPayload{GUID: "populated-legacy-guid", HandleRowID: 19, Date: 84,
			Service: "iMessage", IsFromMe: true, Text: "populated legacy text", HasAttachments: true}},
	} {
		t.Run(want.payload.GUID, func(t *testing.T) {
			var stored messageEventPayload
			var revisionData []byte
			var deletedAt, deletionReason sql.NullString
			if err := st.store.DB().QueryRowContext(ctx, `select guid, handle_rowid, date,
coalesce(service, ''), is_from_me, text, has_attachments, date_edited, date_retracted,
revision_data, deleted_at, deletion_reason from messages where source_rowid = ?`, want.rowID).
				Scan(&stored.GUID, &stored.HandleRowID, &stored.Date, &stored.Service,
					&stored.IsFromMe, &stored.Text, &stored.HasAttachments, &stored.DateEdited,
					&stored.DateRetracted, &revisionData, &deletedAt, &deletionReason); err != nil {
				t.Fatal(err)
			}
			if stored != want.payload || len(revisionData) != 0 || deletedAt.Valid || deletionReason.Valid {
				t.Fatalf("migrated message = %+v, revision=%x, deletion=%v/%v; want %+v",
					stored, revisionData, deletedAt, deletionReason, want.payload)
			}
			var guid, eventType, payloadJSON, observedAt string
			var revisionAt int64
			if err := st.store.DB().QueryRowContext(ctx, `select message_guid, event_type,
revision_at, payload_json, revision_data, observed_at, deleted_at, deletion_reason
from message_events where source_rowid = ?`, want.rowID).
				Scan(&guid, &eventType, &revisionAt, &payloadJSON, &revisionData,
					&observedAt, &deletedAt, &deletionReason); err != nil {
				t.Fatal(err)
			}
			var payload messageEventPayload
			if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
				t.Fatal(err)
			}
			if guid != want.payload.GUID || eventType != "message" || revisionAt != want.payload.Date ||
				payload != want.payload || len(revisionData) != 0 || observedAt != "1970-01-01T00:00:00Z" ||
				deletedAt.Valid || deletionReason.Valid {
				t.Fatalf("seeded legacy event = guid:%q type:%q revision:%d payload:%+v data:%x observed:%q deletion:%v/%v; want %+v",
					guid, eventType, revisionAt, payload, revisionData, observedAt, deletedAt, deletionReason, want.payload)
			}
		})
	}
}
