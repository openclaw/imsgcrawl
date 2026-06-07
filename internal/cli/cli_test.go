package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRunEndToEnd(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "chat.db")
	createMessagesFixture(t, dbPath)
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"help", nil, "imsgcrawl reads local iMessage"},
		{"version", []string{"--version"}, version},
		{"metadata global json", []string{"--json", "metadata"}, `"id": "imsgcrawl"`},
		{"metadata trailing json", []string{"metadata", "--json"}, `"contact-export"`},
		{"status", []string{"--db", dbPath, "--json", "status"}, `"messages": 4`},
		{"contacts export", []string{"--db", dbPath, "--json", "contacts", "export"}, `"display_name": "+15550103"`},
		{"contacts export trailing json", []string{"--db", dbPath, "contacts", "export", "--json"}, `"phone_numbers"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if err := Run(ctx, tc.args, &stdout, &stderr); err != nil {
				t.Fatalf("Run() error = %v stderr=%s", err, stderr.String())
			}
			if !strings.Contains(stdout.String(), tc.want) {
				t.Fatalf("stdout missing %q:\n%s", tc.want, stdout.String())
			}
		})
	}
}

func TestContactsExportShapeAndDedupe(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "chat.db")
	createMessagesFixture(t, dbPath)
	var stdout, stderr bytes.Buffer
	if err := Run(context.Background(), []string{"--db", dbPath, "--json", "contacts", "export"}, &stdout, &stderr); err != nil {
		t.Fatalf("contacts export: %v stderr=%s", err, stderr.String())
	}
	assertContactExportKeys(t, stdout.Bytes())
	var payload struct {
		Contacts []struct {
			DisplayName  string   `json:"display_name"`
			PhoneNumbers []string `json:"phone_numbers"`
			Service      string   `json:"service"`
			Messages     int64    `json:"messages"`
		} `json:"contacts"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json = %s err=%v", stdout.String(), err)
	}
	got := map[string]string{}
	for _, contact := range payload.Contacts {
		if contact.Service != "" || contact.Messages != 0 {
			t.Fatalf("leaked source fields = %#v", contact)
		}
		if len(contact.PhoneNumbers) != 1 {
			t.Fatalf("phone_numbers = %#v", contact.PhoneNumbers)
		}
		got[contact.PhoneNumbers[0]] = contact.DisplayName
	}
	want := map[string]string{
		"0015550100": "Most Recent Name",
		"+15550103":  "+15550103",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("contacts = %#v, want %#v", got, want)
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
	if syncResult.Chats != 4 || syncResult.Participants != 3 || syncResult.ChatMessages != 5 || syncResult.Messages != 4 {
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

	allChatsOut := runOK(t, "--archive", archivePath, "--json", "chats")
	var allChats []struct {
		ChatID string `json:"chat_id"`
	}
	if err := json.Unmarshal([]byte(allChatsOut), &allChats); err != nil {
		t.Fatalf("all chats json = %s err=%v", allChatsOut, err)
	}
	if len(allChats) != 4 {
		t.Fatalf("bare chats should return all chats, got %#v", allChats)
	}

	limitedChatsOut := runOK(t, "--archive", archivePath, "--json", "chats", "--limit", "2")
	var limitedChats []struct {
		ChatID string `json:"chat_id"`
	}
	if err := json.Unmarshal([]byte(limitedChatsOut), &limitedChats); err != nil {
		t.Fatalf("limited chats json = %s err=%v", limitedChatsOut, err)
	}
	if len(limitedChats) != 2 {
		t.Fatalf("limited chats = %#v", limitedChats)
	}

	chatsOut := runOK(t, "--archive", archivePath, "--json", "chats", "--limit", "4")
	var chats []struct {
		ChatID            string `json:"chat_id"`
		Title             string `json:"title"`
		MessageCount      int64  `json:"message_count"`
		LatestMessageDate int64  `json:"latest_message_date"`
	}
	if err := json.Unmarshal([]byte(chatsOut), &chats); err != nil {
		t.Fatalf("chats json = %s err=%v", chatsOut, err)
	}
	if len(chats) != 4 {
		t.Fatalf("chats = %#v", chats)
	}
	if !chatHasMessage(t, chats, "3", "+15550103", 1) || !chatHasMessage(t, chats, "4", "group-chat", 1) {
		t.Fatalf("chats did not preserve chat_message_join rows: %#v", chats)
	}

	messagesOut := runOK(t, "--archive", archivePath, "--json", "messages", "--chat", "2", "--asc")
	var messageRows []struct {
		MessageID string `json:"message_id"`
		GUID      string `json:"guid"`
		ChatID    string `json:"chat_id"`
		Service   string `json:"service"`
		Text      string `json:"text"`
		FromMe    bool   `json:"from_me"`
	}
	if err := json.Unmarshal([]byte(messagesOut), &messageRows); err != nil {
		t.Fatalf("messages json = %s err=%v", messagesOut, err)
	}
	if len(messageRows) != 2 || messageRows[0].Text != "earlier launch note" || messageRows[1].Text != "latest launch note" {
		t.Fatalf("messages = %#v", messageRows)
	}
	if messageRows[1].GUID != "message-three" || !messageRows[1].FromMe || messageRows[1].Service != "SMS" {
		t.Fatalf("source message fields = %#v", messageRows[1])
	}

	attachedOut := runOK(t, "--archive", archivePath, "--json", "messages", "--chat", "3", "--asc")
	var attachedRows []struct {
		MessageID      string `json:"message_id"`
		HasAttachments bool   `json:"has_attachments"`
	}
	if err := json.Unmarshal([]byte(attachedOut), &attachedRows); err != nil {
		t.Fatalf("attached json = %s err=%v", attachedOut, err)
	}
	if len(attachedRows) != 1 || !attachedRows[0].HasAttachments {
		t.Fatalf("attached rows = %#v", attachedRows)
	}

	emptyMessagesOut := runOK(t, "--archive", archivePath, "--json", "messages", "--chat", "999")
	if emptyMessagesOut != "[]\n" {
		t.Fatalf("empty messages output = %q", emptyMessagesOut)
	}

	searchOut := runOK(t, "--archive", archivePath, "--json", "search", "launch")
	var results []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(searchOut), &results); err != nil {
		t.Fatalf("search json = %s err=%v", searchOut, err)
	}
	if len(results) != 2 {
		t.Fatalf("search results = %#v", results)
	}
	for _, result := range results {
		if _, ok := result["snippet"]; !ok {
			t.Fatalf("search result missing snippet = %#v", result)
		}
		if _, ok := result["text"]; ok {
			t.Fatalf("search result leaked full text = %#v", result)
		}
	}

	emptySearchOut := runOK(t, "--archive", archivePath, "--json", "search", "zzznomatchimsgcrawl")
	if emptySearchOut != "[]\n" {
		t.Fatalf("empty search output = %q", emptySearchOut)
	}
}

func TestLimitFlagsAreExplicit(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "chat.db")
	archivePath := filepath.Join(dir, "archive.db")
	createMessagesFixture(t, dbPath)
	_ = runOK(t, "--db", dbPath, "--archive", archivePath, "--json", "sync")

	for _, args := range [][]string{
		{"--archive", archivePath, "chats", "--all", "--limit", "2"},
		{"--archive", archivePath, "messages", "--chat", "1", "--all", "--limit", "2"},
		{"--archive", archivePath, "search", "--all", "--limit", "2", "launch"},
		{"--archive", archivePath, "messages", "--chat", "1", "--limit", "0"},
		{"--archive", archivePath, "search", "--limit", "0", "launch"},
	} {
		var stdout, stderr bytes.Buffer
		err := Run(context.Background(), args, &stdout, &stderr)
		if err == nil || ExitCode(err) != 2 {
			t.Fatalf("Run(%v) expected usage error, got err=%v stdout=%s stderr=%s", args, err, stdout.String(), stderr.String())
		}
	}

	allMessagesOut := runOK(t, "--archive", archivePath, "--json", "messages", "--chat", "2", "--all")
	var allMessages []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(allMessagesOut), &allMessages); err != nil {
		t.Fatalf("all messages json = %s err=%v", allMessagesOut, err)
	}
	if len(allMessages) != 2 {
		t.Fatalf("all messages = %#v", allMessages)
	}

	allSearchOut := runOK(t, "--archive", archivePath, "--json", "search", "--all", "launch")
	var allSearch []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(allSearchOut), &allSearch); err != nil {
		t.Fatalf("all search json = %s err=%v", allSearchOut, err)
	}
	if len(allSearch) != 2 {
		t.Fatalf("all search = %#v", allSearch)
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
	if missing.State != "ok" || !hasWarning(missing.Warnings, "archive has not been synced") {
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
	manifest := controlManifest()
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
		if !hasString(manifest.Privacy.LocalOnlyScopes, want) {
			t.Fatalf("local_only_scopes = %#v, missing %q", manifest.Privacy.LocalOnlyScopes, want)
		}
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
