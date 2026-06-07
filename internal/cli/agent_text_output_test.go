package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveTextOutputIsAgentReadable(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "chat.db")
	archivePath := filepath.Join(dir, "archive.db")
	createMessagesFixture(t, dbPath)
	_ = runOK(t, "--db", dbPath, "--archive", archivePath, "sync")

	chats := runOK(t, "--archive", archivePath, "chats", "--limit", "2")
	assertTextContains(t, chats,
		"Chats: showing 2 of 4, newest first.",
		"More: imsgcrawl chats --limit 4",
		"All: imsgcrawl chats --all",
		"Open: imsgcrawl messages --chat CHAT_ID",
		"chat_id",
	)
	assertNotSecretJSON(t, chats)

	messages := runOK(t, "--archive", archivePath, "messages", "--chat", "2", "--limit", "1")
	assertTextContains(t, messages,
		"Messages for chat 2: showing 1 of 2, newest-first.",
		"More: imsgcrawl messages --chat 2 --limit 2",
		"All: imsgcrawl messages --chat 2 --all",
		"message_id",
		"latest launch note",
	)
	assertNotSecretJSON(t, messages)

	search := runOK(t, "--archive", archivePath, "search", "--limit", "1", "launch")
	assertTextContains(t, search,
		"Search \"launch\": showing 1 of 2.",
		"More: imsgcrawl search --limit 2 \"launch\"",
		"All: imsgcrawl search --all \"launch\"",
		"Open: imsgcrawl messages --chat CHAT_ID",
		"snippet",
	)
	assertNotSecretJSON(t, search)
}

func TestMetadataAndSyncTextOutputIsAgentReadable(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "chat.db")
	archivePath := filepath.Join(dir, "archive.db")
	createMessagesFixture(t, dbPath)

	metadata := runOK(t, "--db", dbPath, "metadata")
	assertTextContains(t, metadata,
		"imsgcrawl metadata",
		"commands:",
		"status",
		"json: add --json for the machine-readable manifest",
	)
	assertNotSecretJSON(t, metadata)

	syncOut := runOK(t, "--db", dbPath, "--archive", archivePath, "sync")
	assertTextContains(t, syncOut,
		"sync complete",
		"chats: 4",
		"messages: 4",
	)
	assertNotSecretJSON(t, syncOut)
}

func assertTextContains(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func assertNotSecretJSON(t *testing.T, got string) {
	t.Helper()
	if strings.Contains(got, `"items"`) || strings.Contains(got, `"schema_version"`) {
		t.Fatalf("text output looks like JSON:\n%s", got)
	}
}
