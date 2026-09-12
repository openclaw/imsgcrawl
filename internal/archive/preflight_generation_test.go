package archive

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/openclaw/imsgcrawl/internal/messages"
)

func TestSyncRejectsArchiveReplacementDuringExtraction(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, "archive.db")
	st, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(root, "foreign.db")
	db, err := sql.Open("sqlite", foreign)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("create table message(text); insert into message values('synthetic foreign data')"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before := captureBundle(t, foreign)
	_, err = syncArchive(ctx, path, filepath.Join(root, "source.db"), false,
		func(context.Context, string) (messages.ArchiveData, error) {
			if err := os.Rename(foreign, path); err != nil {
				t.Fatal(err)
			}
			return messages.ArchiveData{}, nil
		})
	if err == nil {
		t.Fatal("foreign replacement accepted after extraction")
	}
	assertBundle(t, path, before)
}
