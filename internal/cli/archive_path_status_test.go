package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStatusUsesActualArchiveFilename(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.db")
	archive := filepath.Join(dir, "archive.db")
	createMessagesFixture(t, source)
	runOK(t, "--db", source, "--archive", archive, "sync")
	info, err := os.Stat(archive)
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{" ", "\t", "\u00a0"} {
		t.Run(suffix, func(t *testing.T) {
			path := archive + suffix
			output := runOK(t, "--db", source, "--archive", path, "--json", "status")
			var got statusOutput
			if err := json.Unmarshal([]byte(output), &got); err != nil {
				t.Fatal(err)
			}
			if got.State != "ok" || got.Archive == nil || len(got.Warnings) != 0 {
				t.Fatalf("synced archive reported unavailable: %s", output)
			}
			if got.Archive.ArchivePath != path || got.Archive.ArchiveBytes != info.Size() {
				t.Fatalf("archive path/size mismatch: %+v", got.Archive)
			}
		})
	}
}
