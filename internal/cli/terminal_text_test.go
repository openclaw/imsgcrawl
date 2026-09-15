package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/openclaw/imsgcrawl/internal/archive"
	"github.com/openclaw/imsgcrawl/internal/messages"
)

func TestSourceTextEscapesTerminalControls(t *testing.T) {
	dir := t.TempDir()
	source, archive := filepath.Join(dir, "source.db"), filepath.Join(dir, "archive.db")
	createMessagesFixture(t, source)
	db, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	name := "Synthetic\x1b[2J Name"
	body := "orchard \x1b]52;c;c3ludGhldGlj\a \u009b31m café 😀\b\x7f"
	if _, err := db.Exec(`update chat set display_name = ?`, name); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`update message set text = ? where rowid = 1`, body); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	runOK(t, "--db", source, "--archive", archive, "sync")
	for _, args := range [][]string{{"chats"}, {"messages", "--chat", "1"}, {"search", "orchard"}, {"contacts", "export"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			out := runOK(t, append([]string{"--db", source, "--archive", archive}, args...)...)
			for _, r := range out {
				if unicode.IsControl(r) && r != '\n' && r != '\t' {
					t.Fatalf("text output contains active terminal control %U: %q", r, out)
				}
			}
			if !strings.Contains(out, `\x1b`) {
				t.Fatalf("missing visible escape: %q", out)
			}
		})
	}
	var result struct {
		Items []struct{ Text string }
	}
	if err := json.Unmarshal([]byte(runOK(t, "--archive", archive, "--json", "messages", "--chat", "1")), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].Text != body {
		t.Fatalf("JSON changed source text: %#v", result)
	}
}

func TestTerminalControlsInStatusAndSyncPaths(t *testing.T) {
	path := "synthetic\x1b[2J\nErrors:\r\t.db"
	for _, value := range []any{
		archive.SyncResult{SourcePath: path, ArchivePath: path},
		statusOutput{
			Source:   &messages.StatusReport{DatabasePath: path},
			Archive:  &archive.Status{ArchivePath: path},
			Warnings: []string{path}, Errors: []string{path},
		},
	} {
		var out bytes.Buffer
		r := runtime{stdout: &out}
		if err := r.print(value); err != nil {
			t.Fatal(err)
		}
		if strings.ContainsRune(out.String(), '\x1b') || strings.Contains(out.String(), "\nErrors:\r\t.db") || !strings.Contains(out.String(), `synthetic\x1b[2J\nErrors:\r\t.db`) {
			t.Fatalf("unsafe path output: %q", out.String())
		}
	}
}

func TestTerminalEscapesPreserveTextLayout(t *testing.T) {
	input := "café 😀\nnext\tline\r\n\x1b\x00\a\b\x7f\u009b"
	want := "café 😀\nnext\tline\n" + `\x1b\x00\a\b\x7f\u009b`
	if got := normalizeCellText(input); got != want {
		t.Fatalf("normalized text = %q, want %q", got, want)
	}
	if got := wrapCell("\x1b[2Jxx", 6); len(got) != 2 || got[0] != `\x1b[2` || got[1] != "Jxx" {
		t.Fatalf("escape width was not included in wrapping: %#v", got)
	}
}
