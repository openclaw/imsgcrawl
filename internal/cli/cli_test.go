package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/openclaw/crawlkit/control"
)

func TestContactsExportShapeAndDedupe(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "chat.db")
	createMessagesFixture(t, dbPath)
	for _, args := range [][]string{
		{"--db", dbPath, "--json", "contacts", "export"},
		{"--db", dbPath, "contacts", "export", "--json"},
	} {
		t.Run(strings.Join(args[2:], " "), func(t *testing.T) {
			output := runOK(t, args...)
			assertContactExportKeys(t, []byte(output))
			var payload struct {
				Contacts []struct {
					DisplayName  string   `json:"display_name"`
					PhoneNumbers []string `json:"phone_numbers"`
				} `json:"contacts"`
			}
			if err := json.Unmarshal([]byte(output), &payload); err != nil {
				t.Fatalf("json = %s err=%v", output, err)
			}
			got := map[string]string{}
			for _, contact := range payload.Contacts {
				if len(contact.PhoneNumbers) != 1 {
					t.Fatalf("phone_numbers = %#v", contact.PhoneNumbers)
				}
				got[contact.PhoneNumbers[0]] = contact.DisplayName
			}
			want := map[string]string{
				"0015550100": "Most Recent Name",
				"+15550103":  "+15550103",
			}
			if len(payload.Contacts) != len(want) {
				t.Fatalf("contact count = %d, want %d", len(payload.Contacts), len(want))
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("contacts = %#v, want %#v", got, want)
			}
		})
	}
}

func TestArchiveCommandsSyncReadAndSearch(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "chat.db")
	archivePath := filepath.Join(dir, "archive.db")
	createMessagesFixture(t, dbPath)

	syncOut := runOK(t, "--db", dbPath, "--archive", archivePath, "--json", "sync")
	var syncResult struct {
		Handles      int `json:"handles"`
		Chats        int `json:"chats"`
		Participants int `json:"participants"`
		ChatMessages int `json:"chat_messages"`
		Messages     int `json:"messages"`
	}
	if err := json.Unmarshal([]byte(syncOut), &syncResult); err != nil {
		t.Fatalf("sync json = %s err=%v", syncOut, err)
	}
	if syncResult.Chats != 4 || syncResult.Participants != 6 || syncResult.ChatMessages != 5 || syncResult.Messages != 4 {
		t.Fatalf("sync result = %#v", syncResult)
	}

	statusOut := runOK(t, "--db", dbPath, "--archive", archivePath, "--json", "status")
	var status statusOutput
	if err := json.Unmarshal([]byte(statusOut), &status); err != nil {
		t.Fatalf("status json = %s err=%v", statusOut, err)
	}
	if status.Source == nil || status.Archive == nil {
		t.Fatalf("status missing source/archive = %#v", status)
	}
	if status.Source.Messages != status.Archive.Messages || status.Archive.ChatMessages != 5 {
		t.Fatalf("status counts = source %#v archive %#v", status.Source, status.Archive)
	}

	if err := os.Remove(dbPath); err != nil {
		t.Fatal(err)
	}

	wantChats := []chatJSONItem{
		{ChatID: "4", GUID: "chat-four", Title: "Cabinet Group", Kind: "group",
			ChatIdentifier: "group-chat", RoomName: "Cabinet Group", Service: "SMS",
			ParticipantCount: 3, ParticipantHandles: []string{"+15550103", "opaque-handle", "opaque123"},
			MessageCount: 1, LatestMessageDate: 300},
		{ChatID: "3", GUID: "chat-three", Title: "+15550103", Kind: "direct",
			ChatIdentifier: "+15550103", Service: "SMS", ParticipantCount: 1,
			ParticipantHandles: []string{"+15550103"}, MessageCount: 1, LatestMessageDate: 300},
		{ChatID: "2", GUID: "chat-two", Title: "Most Recent Name", Kind: "direct",
			ChatIdentifier: "0015550100", Service: "SMS", ParticipantCount: 1,
			ParticipantHandles: []string{"0015550100"}, MessageCount: 2, LatestMessageDate: 250},
		{ChatID: "1", GUID: "chat-one", Title: "Older Name", Kind: "direct",
			ChatIdentifier: "+15550100", Service: "iMessage", ParticipantCount: 1,
			ParticipantHandles: []string{"+15550100"}, MessageCount: 1, LatestMessageDate: 100},
	}
	for _, tc := range []struct {
		name            string
		args            []string
		limit, returned int
	}{
		{"default chats", nil, defaultChatLimit, 4},
		{"limited chats", []string{"--limit", "2"}, 2, 2},
		{"explicit full chats", []string{"--limit", "4"}, 4, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"--archive", archivePath, "--json", "chats"}, tc.args...)
			output := runOK(t, args...)
			var got chatListJSON
			if err := json.Unmarshal([]byte(output), &got); err != nil {
				t.Fatalf("chats json = %s err=%v", output, err)
			}
			want := chatListJSON{
				SchemaVersion: control.SchemaVersion, AppID: "imsgcrawl", Command: "chats",
				Returned: tc.returned, Total: int64(len(wantChats)), Limit: tc.limit,
				Complete: tc.returned == len(wantChats), Items: wantChats[:tc.returned],
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("chats = %#v, want %#v", got, want)
			}
		})
	}

	messagesOut := runOK(t, "--archive", archivePath, "--json", "messages", "--chat", "2", "--asc")
	var messageRows messageListJSON
	if err := json.Unmarshal([]byte(messagesOut), &messageRows); err != nil {
		t.Fatalf("messages json = %s err=%v", messagesOut, err)
	}
	if messageRows.ChatID != "2" || messageRows.Order != "oldest-first" || messageRows.Returned != 2 || messageRows.Total != 2 || !messageRows.Complete {
		t.Fatalf("message envelope = %#v", messageRows)
	}
	if messageRows.Chat == nil || messageRows.Chat.Title != "Most Recent Name" {
		t.Fatalf("message chat context = %#v", messageRows.Chat)
	}
	if len(messageRows.Items) != 2 || messageRows.Items[0].Text != "earlier launch note" || !strings.Contains(messageRows.Items[1].Text, "full tail marker") {
		t.Fatalf("messages = %#v", messageRows)
	}
	if messageRows.Items[1].GUID != "message-three" || !messageRows.Items[1].FromMe || messageRows.Items[1].Service != "SMS" {
		t.Fatalf("source message fields = %#v", messageRows.Items[1])
	}
	if messageRows.Items[0].SenderLabel != "Most Recent Name" || messageRows.Items[0].SenderHandle != "0015550100" || messageRows.Items[1].SenderLabel != "me" {
		t.Fatalf("sender labels = %#v", messageRows.Items)
	}

	attachedOut := runOK(t, "--archive", archivePath, "--json", "messages", "--chat", "3", "--asc")
	var attachedRows messageListJSON
	if err := json.Unmarshal([]byte(attachedOut), &attachedRows); err != nil {
		t.Fatalf("attached json = %s err=%v", attachedOut, err)
	}
	if len(attachedRows.Items) != 1 || !attachedRows.Items[0].HasAttachments {
		t.Fatalf("attached rows = %#v", attachedRows)
	}
	groupOut := runOK(t, "--archive", archivePath, "--json", "messages", "--chat", "4", "--asc")
	var groupRows messageListJSON
	if err := json.Unmarshal([]byte(groupOut), &groupRows); err != nil {
		t.Fatalf("group json = %s err=%v", groupOut, err)
	}
	if len(groupRows.Items) != 1 || groupRows.Items[0].SenderLabel != "+15550103" {
		t.Fatalf("group sender fallback = %#v", groupRows.Items)
	}

	emptyMessagesOut := runOK(t, "--archive", archivePath, "--json", "messages", "--chat", "999")
	var emptyMessages messageListJSON
	if err := json.Unmarshal([]byte(emptyMessagesOut), &emptyMessages); err != nil {
		t.Fatalf("empty messages json = %s err=%v", emptyMessagesOut, err)
	}
	if emptyMessages.Returned != 0 || emptyMessages.Total != 0 || !emptyMessages.Complete || len(emptyMessages.Items) != 0 {
		t.Fatalf("empty messages output = %#v", emptyMessages)
	}

	searchOut := runOK(t, "--archive", archivePath, "--json", "search", "launch")
	var results searchListJSON
	if err := json.Unmarshal([]byte(searchOut), &results); err != nil {
		t.Fatalf("search json = %s err=%v", searchOut, err)
	}
	if results.Query != "launch" || results.Returned != 2 || results.Total != 2 || !results.Complete || len(results.Items) != 2 {
		t.Fatalf("search results = %#v", results)
	}
	for _, result := range results.Items {
		if _, ok := result["snippet"]; !ok {
			t.Fatalf("search result missing snippet = %#v", result)
		}
		if _, ok := result["chat_title"]; !ok {
			t.Fatalf("search result missing chat title = %#v", result)
		}
		textValue, ok := result["text"]
		if !ok {
			t.Fatalf("search result missing full text = %#v", result)
		}
		if !strings.Contains(string(textValue), "launch note") {
			t.Fatalf("search result text = %s", string(textValue))
		}
	}

	emptySearchOut := runOK(t, "--archive", archivePath, "--json", "search", "zzznomatchimsgcrawl")
	var emptySearch searchListJSON
	if err := json.Unmarshal([]byte(emptySearchOut), &emptySearch); err != nil {
		t.Fatalf("empty search json = %s err=%v", emptySearchOut, err)
	}
	if emptySearch.Returned != 0 || emptySearch.Total != 0 || !emptySearch.Complete || len(emptySearch.Items) != 0 {
		t.Fatalf("empty search output = %#v", emptySearch)
	}
}

func TestArchiveCommandsRequireSync(t *testing.T) {
	for _, args := range [][]string{
		{"--json", "chats"},
		{"--json", "messages", "--chat", "1"},
		{"--json", "search", "hello"},
	} {
		var stdout, stderr bytes.Buffer
		missingPath := filepath.Join(t.TempDir(), "missing.db")
		withArchive := append([]string{"--archive", missingPath}, args...)
		err := Run(context.Background(), withArchive, &stdout, &stderr)
		if err == nil {
			t.Fatalf("Run(%v) expected missing archive error", withArchive)
		}
		if !strings.Contains(err.Error(), "run imsgcrawl sync first") {
			t.Fatalf("err = %v", err)
		}
	}
}

func TestStatusArchiveStates(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "chat.db")
	createMessagesFixture(t, dbPath)

	missingOut := runOK(t, "--db", dbPath, "--archive", filepath.Join(dir, "missing.db"), "--json", "status")
	var missing statusOutput
	if err := json.Unmarshal([]byte(missingOut), &missing); err != nil {
		t.Fatalf("missing status json = %s err=%v", missingOut, err)
	}
	if missing.State != "ok" || missing.Source == nil || missing.Source.Messages != 4 || !hasWarning(missing.Warnings, "archive has not been synced") {
		t.Fatalf("missing archive status = %#v", missing)
	}

	corruptPath := filepath.Join(dir, "corrupt.db")
	if err := os.WriteFile(corruptPath, []byte("not sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}
	corruptOut := runOK(t, "--db", dbPath, "--archive", corruptPath, "--json", "status")
	var corrupt statusOutput
	if err := json.Unmarshal([]byte(corruptOut), &corrupt); err != nil {
		t.Fatalf("corrupt status json = %s err=%v", corruptOut, err)
	}
	if corrupt.State != "archive_error" || len(corrupt.Warnings) == 0 {
		t.Fatalf("corrupt archive status = %#v", corrupt)
	}
}

func TestMetadataAdvertisesCrawlerCommands(t *testing.T) {
	for _, args := range [][]string{{"--json", "metadata"}, {"metadata", "--json"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var manifest control.Manifest
			if err := json.Unmarshal([]byte(runOK(t, args...)), &manifest); err != nil {
				t.Fatal(err)
			}
			if manifest.ID != "imsgcrawl" {
				t.Fatalf("metadata id = %q", manifest.ID)
			}
			command, ok := manifest.Commands["contact-export"]
			if !ok {
				t.Fatalf("commands = %#v", manifest.Commands)
			}
			if command.Mutates || !command.JSON {
				t.Fatalf("contact-export command = %#v", command)
			}
			want := []string{"imsgcrawl", "--json", "contacts", "export"}
			if !reflect.DeepEqual(command.Argv, want) {
				t.Fatalf("argv = %#v, want %#v", command.Argv, want)
			}
			for _, name := range []string{"sync", "chats", "messages", "search"} {
				command, ok := manifest.Commands[name]
				if !ok {
					t.Fatalf("missing command %q in %#v", name, manifest.Commands)
				}
				if !command.JSON {
					t.Fatalf("%s command is not JSON = %#v", name, command)
				}
			}
			if !manifest.Commands["sync"].Mutates {
				t.Fatalf("sync should be marked mutating = %#v", manifest.Commands["sync"])
			}
			for _, want := range []string{"message-archive", "message-text-search"} {
				if !slices.Contains(manifest.Privacy.LocalOnlyScopes, want) {
					t.Fatalf("local_only_scopes = %#v, missing %q", manifest.Privacy.LocalOnlyScopes, want)
				}
			}
		})
	}
}

func TestRunUsageErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), []string{"bogus"}, &stdout, &stderr)
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("expected usage exit, got err=%v code=%d", err, ExitCode(err))
	}
	if ExitCode(nil) != 0 {
		t.Fatal("nil exit code should be zero")
	}
	if ExitCode(errors.New("plain")) != 1 {
		t.Fatal("plain error exit code should be one")
	}
}

func assertContactExportKeys(t *testing.T, data []byte) {
	t.Helper()
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	contactsJSON, ok := root["contacts"]
	if !ok || len(root) != 1 {
		t.Fatalf("root keys = %#v, want only contacts", root)
	}
	var contacts []map[string]json.RawMessage
	if err := json.Unmarshal(contactsJSON, &contacts); err != nil {
		t.Fatal(err)
	}
	for _, contact := range contacts {
		if _, ok := contact["display_name"]; !ok {
			t.Fatalf("contact keys = %#v, missing display_name", contact)
		}
		if _, ok := contact["phone_numbers"]; !ok {
			t.Fatalf("contact keys = %#v, missing phone_numbers", contact)
		}
		if len(contact) != 2 {
			t.Fatalf("contact keys = %#v, want only display_name and phone_numbers", contact)
		}
	}
}
