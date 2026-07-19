package messages

import (
	"context"
	"database/sql"
	"testing"

	"howett.net/plist"
	_ "modernc.org/sqlite"
)

func TestExtractMessagesCapturesRevisionMetadata(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	_, err = db.Exec(`
create table message (
  rowid integer primary key, guid text, handle_id integer, date integer, service text,
  is_from_me integer, text text, attributedBody blob, date_edited integer,
  date_retracted integer, message_summary_info blob
);
create table message_attachment_join(message_id integer, attachment_id integer);`)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := plist.Marshal(map[string]any{
		"otr": map[string]any{"0": map[string]any{}},
		"rp":  []any{int64(0)},
	}, plist.BinaryFormat)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`insert into message values(1, 'edited-guid', 2, 3, 'iMessage', 1, 'current text', x'', 4, 5, ?)`, summary); err != nil {
		t.Fatal(err)
	}
	rows, err := extractMessages(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].DateEdited != 4 || rows[0].DateRetracted != 5 || !rows[0].HasUnsentParts || !rows[0].FullyUnsent || rows[0].RevisionIdentity == "" || !rows[0].DateEditedAvailable || !rows[0].DateRetractedAvailable || !rows[0].RevisionDataAvailable {
		t.Fatalf("revision-aware message = %#v", rows)
	}
}

func TestExtractMessagesMarksLegacyRevisionColumnsUnavailable(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`create table message (
rowid integer primary key, guid text, handle_id integer, date integer, service text,
is_from_me integer, text text, attributedBody blob
); create table message_attachment_join(message_id integer, attachment_id integer);
insert into message values(1, 'legacy-guid', 0, 1, 'iMessage', 0, 'legacy', x'');`); err != nil {
		t.Fatal(err)
	}
	rows, err := extractMessages(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].DateEditedAvailable || rows[0].DateRetractedAvailable || rows[0].RevisionDataAvailable {
		t.Fatalf("legacy revision availability = %#v", rows)
	}
}

func TestExtractMessagesMarksAttributedBodyAsCurrent(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`create table message (
rowid integer primary key, guid text, handle_id integer, date integer, service text,
is_from_me integer, text text, attributedBody blob, date_edited integer,
date_retracted integer, message_summary_info blob
); create table message_attachment_join(message_id integer, attachment_id integer);`); err != nil {
		t.Fatal(err)
	}
	summary, err := plist.Marshal(map[string]any{
		"otr": map[string]any{
			"0": map[string]any{"lo": int64(0), "le": int64(1)},
			"1": map[string]any{"lo": int64(1), "le": int64(4)},
		},
		"ec": map[string]any{"0": []any{map[string]any{"d": int64(2), "t": makeStreamtypedAttributedBody("LONGER")}}},
	}, plist.BinaryFormat)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`insert into message values(1, 'edited-guid', 0, 1, 'iMessage', 0, '', ?, 2, 0, ?)`,
		makeStreamtypedAttributedBody("LONGERtail"), summary); err != nil {
		t.Fatal(err)
	}
	rows, err := extractMessages(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Text != "LONGERtail" || !rows[0].TextAvailable || !rows[0].TextIsCurrent {
		t.Fatalf("current attributed body = %#v", rows)
	}
}

func TestExtractMessagesHidesFallbackForMalformedKnownEdit(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`create table message (
rowid integer primary key, guid text, handle_id integer, date integer, service text,
is_from_me integer, text text, attributedBody blob, date_edited integer,
date_retracted integer, message_summary_info blob
); create table message_attachment_join(message_id integer, attachment_id integer);
insert into message values(1, 'edited-guid', 0, 1, 'iMessage', 0,
'withdrawn fallback', x'', 2, 0, x'6e6f74206120706c697374');`); err != nil {
		t.Fatal(err)
	}
	rows, err := extractMessages(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].TextAvailable {
		t.Fatalf("malformed known edit exposed fallback = %#v", rows)
	}
}

func TestExtractDeletedGUIDsIsOptionalAndDeduplicated(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	missing, err := extractDeletedGUIDs(ctx, db, "sync_deleted_messages")
	if err != nil || len(missing) != 0 {
		t.Fatalf("missing delete feed = %#v, %v", missing, err)
	}
	if _, err := db.Exec(`create table sync_deleted_messages(guid text); insert into sync_deleted_messages values(' b '), ('a'), ('a'), ('')`); err != nil {
		t.Fatal(err)
	}
	got, err := extractDeletedGUIDs(ctx, db, "sync_deleted_messages")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("deleted GUIDs = %#v", got)
	}
}
