package archive

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestForeignArchiveRejectedBeforeMutation(t *testing.T) {
	for _, journal := range []string{"DELETE", "WAL"} {
		t.Run(journal, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "source.db")
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec("pragma journal_mode=" + journal + "; create table message(guid text); insert into message values('synthetic')"); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, 0o644); err != nil {
				t.Fatal(err)
			}
			before := captureBundle(t, path)
			link := filepath.Join(filepath.Dir(path), "alias.db")
			if err := os.Link(path, link); err != nil {
				t.Fatal(err)
			}
			for _, open := range []func(context.Context, string) (*Store, error){Open, OpenExisting} {
				if st, err := open(context.Background(), path); err == nil {
					_ = st.Close()
					t.Fatal("foreign database accepted")
				}
				if st, err := open(context.Background(), link); err == nil {
					_ = st.Close()
					t.Fatal("hardlinked foreign database accepted")
				}
				assertBundle(t, path, before)
			}
			if _, err := Sync(context.Background(), path, filepath.Join(t.TempDir(), "missing.db"), false); err == nil {
				t.Fatal("foreign archive accepted for sync")
			}
			assertBundle(t, path, before)
		})
	}
}

func TestArchiveFilenameIsNotReinterpretedAsSQLiteURI(t *testing.T) {
	t.Chdir(t.TempDir())
	st, err := Open(context.Background(), "file:literal.db")
	if err != nil {
		t.Fatal(err)
	}
	_ = st.Close()
	if _, err := os.Stat("file:literal.db"); err != nil {
		t.Fatalf("archive path was interpreted as a URI: %v", err)
	}
	if _, err := os.Stat("literal.db"); !os.IsNotExist(err) {
		t.Fatalf("unexpected URI target: %v", err)
	}
}

func TestSyncRejectsSourceAliasesBeforeExtraction(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.db")
	if err := os.WriteFile(source, []byte("synthetic source"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.WriteFile(source+suffix, []byte("synthetic sidecar"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	symlink, hardlink := filepath.Join(root, "symlink.db"), filepath.Join(root, "hardlink.db")
	if err := os.Symlink(source, symlink); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(source, hardlink); err != nil {
		t.Fatal(err)
	}
	before := captureBundle(t, source)
	for _, archive := range []string{source, symlink, hardlink, source + "-wal", source + "-shm"} {
		if _, err := Sync(context.Background(), archive, source, true); err == nil {
			t.Fatalf("accepted alias %s", filepath.Base(archive))
		}
		assertBundle(t, source, before)
		if _, err := os.Stat(archive + ".sync.lock"); !os.IsNotExist(err) {
			t.Fatalf("lock created before alias rejection: %v", err)
		}
	}
}

func TestSyncRejectsCanonicalSidecarsOfSymlinkedSource(t *testing.T) {
	root := t.TempDir()
	source, alias := filepath.Join(root, "source.db"), filepath.Join(root, "alias.db")
	if err := os.WriteFile(source, []byte("synthetic source"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(source, alias); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if err := os.WriteFile(source+suffix, []byte("synthetic sidecar"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	before := captureBundle(t, source)
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if err := rejectSourceOverlap(source+suffix, alias); err == nil {
			t.Fatalf("canonical source sidecar %s accepted", suffix)
		}
		if _, err := Sync(context.Background(), source+suffix, alias, true); err == nil {
			t.Fatalf("sync accepted canonical source sidecar %s", suffix)
		}
		assertBundle(t, source, before)
	}
}

type bundleEntry struct {
	data []byte
	mode os.FileMode
}

func TestOpenAllowsOrdinaryEmptyArchiveFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.db")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	st.Close()
}

func TestOpenRejectsSidecarsBesideEmptyArchive(t *testing.T) {
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		t.Run(suffix, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "archive.db")
			if err := os.WriteFile(path, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path+suffix, []byte("synthetic orphan sidecar"), 0o600); err != nil {
				t.Fatal(err)
			}
			before := captureBundle(t, path)
			if st, err := Open(context.Background(), path); err == nil {
				st.Close()
				t.Error("orphan sidecar beside empty archive accepted")
			}
			assertBundle(t, path, before)
		})
	}
}

func TestOpenRejectsUnownedSidecarsBeforeCreatingArchive(t *testing.T) {
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		t.Run(suffix, func(t *testing.T) {
			root := t.TempDir()
			foreign, archive := filepath.Join(root, "foreign.db"), filepath.Join(root, "archive.db")
			if err := os.WriteFile(foreign, []byte("caller-owned bytes"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(foreign, archive+suffix); err != nil {
				t.Fatal(err)
			}
			before := captureBundle(t, foreign)
			if st, err := Open(context.Background(), archive); err == nil {
				st.Close()
				t.Fatal("unowned sidecar accepted")
			}
			assertBundle(t, foreign, before)
			if _, err := os.Stat(archive); !os.IsNotExist(err) {
				t.Fatalf("archive created before sidecar rejection: %v", err)
			}
		})
	}
}
func captureBundle(t *testing.T, path string) map[string]bundleEntry {
	t.Helper()
	result := map[string]bundleEntry{}
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		data, err := os.ReadFile(path + suffix)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path + suffix)
		if err != nil {
			t.Fatal(err)
		}
		result[suffix] = bundleEntry{data, info.Mode()}
	}
	return result
}

func assertBundle(t *testing.T, path string, before map[string]bundleEntry) {
	t.Helper()
	after := captureBundle(t, path)
	if len(before) != len(after) {
		t.Fatal("source sidecar inventory changed")
	}
	for suffix, want := range before {
		got, exists := after[suffix]
		if !exists || !bytes.Equal(want.data, got.data) || want.mode != got.mode {
			t.Fatalf("source bytes or mode changed for %q", suffix)
		}
	}
}
