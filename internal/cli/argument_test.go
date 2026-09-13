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

func TestSearchPreservesLiteralFlagsAfterTerminator(t *testing.T) {
	dir := t.TempDir()
	source, archive := filepath.Join(dir, "source.db"), filepath.Join(dir, "archive.db")
	createMessagesFixture(t, source)
	db, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("update message set text = 'literal --help and --json' where rowid = 1"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	runOK(t, "--db", source, "--archive", archive, "sync")
	for _, query := range []string{"--help", "--json", "--help --json"} {
		t.Run(query, func(t *testing.T) {
			text := runOK(t, "--archive", archive, "search", "--", query)
			if !strings.Contains(text, "literal --help and --json") || strings.Contains(text, "Usage:") {
				t.Fatalf("literal query was treated as flags: %s", text)
			}
			output := runOK(t, "--archive", archive, "--json", "search", "--", query)
			var got searchListJSON
			if err := json.Unmarshal([]byte(output), &got); err != nil {
				t.Fatal(err)
			}
			if got.Query != query || got.Returned != 1 {
				t.Fatalf("literal query not preserved: %s", output)
			}
		})
	}
}

func TestGlobalPathFlagsAllowTopLevelHelp(t *testing.T) {
	for _, help := range []string{"--help", "-help", "-h"} {
		var out, stderr bytes.Buffer
		err := Run(context.Background(), []string{"--db", "missing-source.db", "--archive", "missing-archive.db", help}, &out, &stderr)
		if err != nil || !strings.Contains(out.String(), "Usage:") || stderr.Len() != 0 {
			t.Fatalf("global flags with %s: error=%v stdout=%q stderr=%q", help, err, out.String(), stderr.String())
		}
	}
}
