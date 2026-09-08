package archive

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/openclaw/imsgcrawl/internal/messages"
)

func TestArchiveOpenValidatesActualSQLiteFilename(t *testing.T) {
	for _, operation := range []string{"Open", "OpenExisting", "Sync"} {
		t.Run(operation, func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "a", "b"), 0o700); err != nil {
				t.Fatal(err)
			}
			valid := filepath.Join(root, "a", "archive.db")
			archive, err := Open(ctx, valid)
			if err != nil {
				t.Fatal(err)
			}
			if err := archive.Close(); err != nil {
				t.Fatal(err)
			}
			foreign := filepath.Join(root, "archive.db")
			db, err := sql.Open("sqlite", foreign)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec("pragma journal_mode=DELETE; create table foreign_marker(value text); insert into foreign_marker values ('synthetic-only')"); err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(foreign, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(root, "a", "b"), filepath.Join(root, "alias")); err != nil {
				t.Fatal(err)
			}
			// Join would erase the static spelling whose two interpretations differ.
			raw := root + "/alias/../archive.db"
			actual, err := filepath.Abs(raw)
			if err != nil || actual != foreign {
				t.Fatalf("actual SQLite filename = %q: %v", actual, err)
			}
			rawInfo, err := os.Stat(raw)
			if err != nil {
				t.Fatal(err)
			}
			validInfo, err := os.Stat(valid)
			if err != nil || !os.SameFile(rawInfo, validInfo) {
				t.Fatalf("raw path did not select valid fixture: %v", err)
			}
			before, validBefore := captureBundle(t, foreign), captureBundle(t, valid)
			switch operation {
			case "Open", "OpenExisting":
				open := Open
				if operation == "OpenExisting" {
					open = OpenExisting
				}
				st, err := open(ctx, raw)
				if st != nil {
					st.Close()
				}
				if err == nil {
					t.Error("foreign actual filename accepted")
				}
			case "Sync":
				_, err := syncArchive(ctx, raw, filepath.Join(root, "source.db"), false,
					func(context.Context, string) (messages.ArchiveData, error) {
						t.Error("foreign actual filename reached extraction")
						return fixtureArchiveData(), nil
					})
				if err == nil {
					t.Error("foreign actual filename accepted")
				}
			}
			assertBundle(t, foreign, before)
			assertBundle(t, valid, validBefore)
		})
	}
}

func TestSyncPreservesArchivePathSpelling(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("nested", 0o700); err != nil {
		t.Fatal(err)
	}
	path := "nested/../archive.db"
	result, err := syncArchive(context.Background(), path, filepath.Join(t.TempDir(), "source.db"), false,
		func(context.Context, string) (messages.ArchiveData, error) {
			return fixtureArchiveData(), nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if result.ArchivePath != path {
		t.Fatalf("display path = %q, want %q", result.ArchivePath, path)
	}
}

func TestArchiveOpenPathCompatibility(t *testing.T) {
	for _, kind := range []string{"symlink", "dangling"} {
		t.Run(kind, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if kind == "symlink" {
				st, err := Open(context.Background(), "target.db")
				if err != nil {
					t.Fatal(err)
				}
				st.Close()
			}
			if err := os.Symlink("target.db", "alias.db"); err != nil {
				t.Fatal(err)
			}
			for _, open := range []func(context.Context, string) (*Store, error){Open, OpenExisting} {
				st, err := open(context.Background(), "alias.db")
				if err != nil {
					t.Fatal(err)
				}
				if st.path != "alias.db" {
					t.Errorf("display path changed: %q", st.path)
				}
				st.Close()
			}
			if _, err := os.Stat("target.db"); err != nil {
				t.Fatal(err)
			}
		})
	}
}
