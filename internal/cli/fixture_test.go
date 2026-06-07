package cli

import (
	"bytes"
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func runOK(t *testing.T, args ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if err := Run(context.Background(), args, &stdout, &stderr); err != nil {
		t.Fatalf("Run(%v) error = %v stderr=%s stdout=%s", args, err, stderr.String(), stdout.String())
	}
	return stdout.String()
}

func chatHasMessage(t *testing.T, chats []chatJSONItem, chatID, title string, messages int64) bool {
	t.Helper()
	for _, chat := range chats {
		if chat.ChatID == chatID && chat.Title == title && chat.MessageCount == messages {
			return true
		}
	}
	return false
}

func hasWarning(warnings []string, want string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, want) {
			return true
		}
	}
	return false
}

func hasString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func createMessagesFixture(t *testing.T, path string) {
	t.Helper()
	longLaunchNote := "latest launch note with candles budget and tariffs. " + strings.Repeat("This sentence keeps going so transcript output must stay whole. ", 3) + "full tail marker"
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	schema := []string{
		`create table handle (ROWID integer primary key, id text not null, service text not null, uncanonicalized_id text)`,
		`create table chat (ROWID integer primary key, guid text not null, display_name text, chat_identifier text, service_name text, room_name text, is_archived integer)`,
		`create table chat_handle_join (chat_id integer, handle_id integer)`,
		`create table message (ROWID integer primary key, guid text not null, handle_id integer, date integer, service text, is_from_me integer, text text, attributedBody blob)`,
		`create table chat_message_join (chat_id integer, message_id integer)`,
		`create table message_attachment_join (message_id integer, attachment_id integer)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	inserts := []string{
		`insert into handle(rowid, id, service, uncanonicalized_id) values (1, '+15550100', 'iMessage', '')`,
		`insert into handle(rowid, id, service, uncanonicalized_id) values (2, '0015550100', 'SMS', '')`,
		`insert into handle(rowid, id, service, uncanonicalized_id) values (3, 'person@example.test', 'iMessage', '')`,
		`insert into handle(rowid, id, service, uncanonicalized_id) values (4, '+15550103', 'SMS', '')`,
		`insert into handle(rowid, id, service, uncanonicalized_id) values (5, 'opaque-handle', 'SMS', '')`,
		`insert into handle(rowid, id, service, uncanonicalized_id) values (6, 'opaque123', 'SMS', '')`,
		`insert into chat(rowid, guid, display_name, chat_identifier, service_name, room_name, is_archived) values (1, 'chat-one', 'Older Name', '+15550100', 'iMessage', '', 0)`,
		`insert into chat(rowid, guid, display_name, chat_identifier, service_name, room_name, is_archived) values (2, 'chat-two', 'Most Recent Name', '0015550100', 'SMS', '', 0)`,
		`insert into chat(rowid, guid, display_name, chat_identifier, service_name, room_name, is_archived) values (3, 'chat-three', '', '+15550103', 'SMS', '', 0)`,
		`insert into chat(rowid, guid, display_name, chat_identifier, service_name, room_name, is_archived) values (4, 'chat-four', '', 'group-chat', 'SMS', 'Cabinet Group', 0)`,
		`insert into chat_handle_join(chat_id, handle_id) values (1, 1)`,
		`insert into chat_handle_join(chat_id, handle_id) values (2, 2)`,
		`insert into chat_handle_join(chat_id, handle_id) values (3, 4)`,
		`insert into chat_handle_join(chat_id, handle_id) values (4, 4)`,
		`insert into chat_handle_join(chat_id, handle_id) values (4, 5)`,
		`insert into chat_handle_join(chat_id, handle_id) values (4, 6)`,
		`insert into message(rowid, guid, handle_id, date, service, is_from_me, text, attributedBody) values (1, 'message-one', 1, 100, 'iMessage', 0, 'older hello', null)`,
		`insert into message(rowid, guid, handle_id, date, service, is_from_me, text, attributedBody) values (2, 'message-two', 2, 200, 'SMS', 0, '', null)`,
		`insert into message(rowid, guid, handle_id, date, service, is_from_me, text, attributedBody) values (3, 'message-three', 2, 250, 'SMS', 1, 'latest launch note', null)`,
		`insert into message(rowid, guid, handle_id, date, service, is_from_me, text, attributedBody) values (4, 'message-four', 4, 300, 'SMS', 0, 'group fallback row', null)`,
		`insert into chat_message_join(chat_id, message_id) values (1, 1)`,
		`insert into chat_message_join(chat_id, message_id) values (2, 2)`,
		`insert into chat_message_join(chat_id, message_id) values (2, 3)`,
		`insert into chat_message_join(chat_id, message_id) values (3, 4)`,
		`insert into chat_message_join(chat_id, message_id) values (4, 4)`,
		`insert into message_attachment_join(message_id, attachment_id) values (4, 42)`,
	}
	for _, stmt := range inserts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`update message set attributedBody = ? where rowid = 2`, makeFixtureAttributedBody("earlier launch note")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`update message set text = ? where rowid = 3`, longLaunchNote); err != nil {
		t.Fatal(err)
	}
}

func makeFixtureAttributedBody(text string) []byte {
	var out []byte
	out = append(out, "\x04\x0bstreamtyped\x81\xe8\x03\x84\x01@\x84\x84\x84"...)
	out = append(out, "\x12NSAttributedString"...)
	out = append(out, "\x00\x84\x84\x08NSObject\x00\x85\x92\x84\x84\x84\x08NSString\x01\x94"...)
	out = append(out, "\x84\x01+"...)
	out = append(out, 0x81, byte(len(text)), 0x92, 0x00)
	out = append(out, text...)
	out = append(out, 0x86)
	return out
}
