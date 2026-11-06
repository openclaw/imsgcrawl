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

func TestExtractMessagesCurrentText(t *testing.T) {
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
	for _, test := range []struct {
		name          string
		fallback      string
		body, summary []byte
		wantText      string
		wantAvailable bool
		wantCurrent   bool
	}{
		{
			name: "current body with revision summary",
			body: makeStreamtypedAttributedBody("LONGERtail"), summary: summary,
			wantText: "LONGERtail", wantAvailable: true, wantCurrent: true,
		},
		{
			name:     "fallback with malformed revision summary",
			fallback: "withdrawn fallback", body: []byte{}, summary: []byte("not a plist"),
		},
		{
			name: "current body with malformed revision summary",
			body: makeStreamtypedAttributedBody("known current"), summary: []byte("not a plist"),
			wantText: "known current", wantAvailable: true, wantCurrent: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
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
			if _, err := db.Exec(`insert into message values(1, 'edited-guid', 0, 1, 'iMessage', 0, ?, ?, 2, 0, ?)`,
				test.fallback, test.body, test.summary); err != nil {
				t.Fatal(err)
			}
			rows, err := extractMessages(context.Background(), db)
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 1 {
				t.Fatalf("extracted messages = %#v", rows)
			}
			got := rows[0]
			if got.TextAvailable != test.wantAvailable || got.TextIsCurrent != test.wantCurrent ||
				(test.wantAvailable && got.Text != test.wantText) {
				t.Fatalf("current text = %q, available=%v, current=%v; want %q, available=%v, current=%v",
					got.Text, got.TextAvailable, got.TextIsCurrent, test.wantText, test.wantAvailable, test.wantCurrent)
			}
		})
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
