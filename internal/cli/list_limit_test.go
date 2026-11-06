package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestLimitFlagsAreExplicit(t *testing.T) {
	dir := t.TempDir()
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"chats", "--all", "--limit", "2"}, "use either --all or --limit"},
		{[]string{"chats", "--limit", "0"}, "chats --limit must be positive"},
		{[]string{"messages", "--chat", "1", "--all", "--limit", "2"}, "use either --all or --limit"},
		{[]string{"search", "--all", "--limit", "2", "launch"}, "use either --all or --limit"},
		{[]string{"messages", "--chat", "1", "--limit", "0"}, "messages --limit must be positive"},
		{[]string{"search", "--limit", "0", "launch"}, "search --limit must be positive"},
	} {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			args := append([]string{"--db", filepath.Join(dir, "missing-source.db"), "--archive", filepath.Join(dir, "missing-archive.db")}, test.args...)
			var stdout, stderr bytes.Buffer
			err := Run(context.Background(), args, &stdout, &stderr)
			if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected usage error %q, got err=%v stdout=%s stderr=%s", test.want, err, stdout.String(), stderr.String())
			}
		})
	}
}

func TestListAllExceedsDefaultLimits(t *testing.T) {
	dir := t.TempDir()
	source, archive := filepath.Join(dir, "source.db"), filepath.Join(dir, "archive.db")
	chatCount, messageCount := defaultChatLimit+1, max(defaultMessageLimit, defaultSearchLimit)+1
	createListLimitFixture(t, source, chatCount, messageCount)
	runOK(t, "--db", source, "--archive", archive, "sync")

	for _, test := range []struct {
		args         []string
		total, limit int
	}{
		{[]string{"chats"}, chatCount, defaultChatLimit},
		{[]string{"messages", "--chat", "1"}, messageCount, defaultMessageLimit},
		{[]string{"search", "pagination"}, messageCount, defaultSearchLimit},
	} {
		t.Run(test.args[0], func(t *testing.T) {
			for _, all := range []bool{false, true} {
				args := []string{"--archive", archive, "--json", test.args[0]}
				wantReturned, wantLimit := test.limit, test.limit
				if all {
					args = append(args, "--all")
					wantReturned, wantLimit = test.total, 0
				}
				args = append(args, test.args[1:]...)
				output := runOK(t, args...)
				var got struct {
					Returned int               `json:"returned"`
					Total    int               `json:"total"`
					Limit    int               `json:"limit"`
					Complete bool              `json:"complete"`
					Items    []json.RawMessage `json:"items"`
				}
				if err := json.Unmarshal([]byte(output), &got); err != nil {
					t.Fatal(err)
				}
				if got.Returned != wantReturned || len(got.Items) != wantReturned ||
					got.Total != test.total || got.Limit != wantLimit || got.Complete != all {
					t.Fatalf("all=%v: returned=%d items=%d total=%d limit=%d complete=%v; want %d/%d/%d/%d/%v",
						all, got.Returned, len(got.Items), got.Total, got.Limit, got.Complete,
						wantReturned, wantReturned, test.total, wantLimit, all)
				}
			}
		})
	}
}

func createListLimitFixture(t *testing.T, path string, chats, messages int) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`
create table handle(rowid integer primary key, id text, service text, uncanonicalized_id text);
create table chat(rowid integer primary key, guid text, display_name text, chat_identifier text, service_name text, room_name text, is_archived integer);
create table chat_handle_join(chat_id integer, handle_id integer);
create table message(rowid integer primary key, guid text, handle_id integer, date integer, service text, is_from_me integer, text text, attributedBody blob);
create table chat_message_join(chat_id integer, message_id integer);
create table message_attachment_join(message_id integer, attachment_id integer);`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`with recursive ids(id) as (
  select 1 union all select id+1 from ids where id < ?
)
insert into chat(rowid, guid, display_name)
select id, 'synthetic-chat-' || id, 'Synthetic chat ' || id from ids`, chats); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`with recursive ids(id) as (
  select 1 union all select id+1 from ids where id < ?
)
insert into message(rowid, guid, date, text)
select id, 'synthetic-message-' || id, id, 'pagination marker ' || id from ids`, messages); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`insert into chat_message_join select 1, rowid from message`); err != nil {
		t.Fatal(err)
	}
}
