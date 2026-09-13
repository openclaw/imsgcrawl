package messages

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/openclaw/crawlkit/store"
)

func TestSnapshotPathCopiesSQLiteBundle(t *testing.T) {
	source := filepath.Join(t.TempDir(), "chat.db")
	db, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`pragma journal_mode=WAL; pragma wal_autocheckpoint=0;
create table fixture(value text); insert into fixture values('committed in WAL')`); err != nil {
		t.Fatal(err)
	}
	before := map[string][]byte{}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		before[suffix], err = os.ReadFile(source + suffix)
		if err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := SnapshotPathContext(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	private, err := store.OpenReadOnly(context.Background(), snapshot.Path)
	if err != nil {
		t.Fatal(err)
	}
	var got string
	if err := private.DB().QueryRow("select value from fixture").Scan(&got); err != nil || got != "committed in WAL" {
		t.Fatalf("snapshot value = %q, err = %v", got, err)
	}
	_ = private.Close()
	for suffix, want := range before {
		after, err := os.ReadFile(source + suffix)
		if err != nil || !bytes.Equal(after, want) {
			t.Fatalf("source %q changed: %v", suffix, err)
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

func TestSnapshotPathRejectsCorruptionAndCancellation(t *testing.T) {
	source := filepath.Join(t.TempDir(), "chat.db")
	if err := os.WriteFile(source, []byte("not a database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := SnapshotPathContext(context.Background(), source); err == nil {
		t.Fatal("corrupt source accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := SnapshotPathContext(ctx, source); err == nil {
		t.Fatal("cancelled snapshot accepted")
	}
}
