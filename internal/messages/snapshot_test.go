package messages

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotPathCopiesSQLiteBundle(t *testing.T) {
	source := filepath.Join(t.TempDir(), "chat.db")
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.WriteFile(source+suffix, []byte("fixture"+suffix), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := SnapshotPath(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		data, err := os.ReadFile(snapshot.Path + suffix)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "fixture"+suffix {
			t.Fatalf("snapshot %s = %q", suffix, data)
		}
	}
	root := snapshot.root
	if err := snapshot.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("snapshot root should be removed: %v", err)
	}
}
