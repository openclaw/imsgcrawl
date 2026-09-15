package messages

import (
	"context"
	"database/sql"
	"testing"
)

func TestExtractMessagesRejectsMalformedCurrentBody(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`create table message (
rowid integer primary key, guid text, handle_id integer, date integer, service text,
is_from_me integer, text text, attributedBody blob, date_edited integer
); create table message_attachment_join(message_id integer, attachment_id integer);`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`insert into message values(1, 'synthetic-edited', 0, 1, 'iMessage', 0, '', ?, 2)`,
		makeStreamtypedAttributedBody("prefix\xfftail")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`insert into message values(2, 'synthetic-unedited', 0, 1, 'iMessage', 0, '', ?, 0)`,
		makeStreamtypedAttributedBody("prefix\xfftail")); err != nil {
		t.Fatal(err)
	}
	rows, err := extractMessages(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("extracted messages = %#v", rows)
	}
	for _, row := range rows {
		if row.Text != "" || row.TextIsCurrent || row.TextAvailable {
			t.Fatalf("malformed body accepted as current text: %#v", rows)
		}
	}
}
