package archive

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/openclaw/imsgcrawl/internal/messages"
)

func TestArchiveFilenameWhitespace(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, raw := range []string{"archive.db ", "archive.db\t", "archive.db\u00a0", " archive.db "} {
		got, err := archiveFilename(raw)
		want := filepath.Join(mustWorkingDirectory(t), "archive.db")
		if raw[0] == ' ' {
			want = filepath.Join(mustWorkingDirectory(t), " archive.db")
		}
		if err != nil || got != want {
			t.Fatalf("filename %q = %q, %v; want %q", raw, got, err, want)
		}
		if again, err := archiveFilename(got); err != nil || again != got {
			t.Fatalf("filename is not stable: %q, %v", again, err)
		}
	}
	for _, raw := range []string{"new-parent/. ", "new-parent/.. ", "new-parent/ \t"} {
		for _, open := range []func(context.Context, string) (*Store, error){Open, OpenExisting} {
			if st, err := open(context.Background(), raw); err == nil {
				st.Close()
				t.Fatalf("unstable filename accepted: %q", raw)
			}
		}
		if _, err := Sync(context.Background(), raw, "source.db", false); err == nil {
			t.Fatalf("unstable sync filename accepted: %q", raw)
		}
		if _, err := os.Stat("new-parent"); !os.IsNotExist(err) {
			t.Fatalf("unstable filename created a parent: %v", err)
		}
	}
}

func TestArchiveWhitespaceRejectsSourceTarget(t *testing.T) {
	for _, journal := range []string{"DELETE", "WAL"} {
		for _, suffix := range []string{" ", "\t", "\u00a0"} {
			for _, decoy := range []string{"absent", "current", "legacy"} {
				for _, operation := range []string{"Open", "OpenExisting", "Sync"} {
					t.Run(journal+"/"+suffix+"/"+decoy+"/"+operation, func(t *testing.T) {
						ctx := context.Background()
						source := filepath.Join(t.TempDir(), "source.db")
						db := createWhitespaceMessagesFixture(t, source, journal)
						defer db.Close()
						raw := source + suffix
						if decoy != "absent" {
							createWhitespaceArchiveFixture(t, raw, decoy == "legacy")
						}
						before, decoyBefore := captureBundle(t, source), captureBundle(t, raw)
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
								t.Fatal("source database accepted as an archive")
							}
						case "Sync":
							if _, err := Sync(ctx, raw, source, false); err == nil {
								t.Fatal("source overlap accepted")
							}
							_, err := syncArchive(ctx, raw, source, false,
								func(context.Context, string) (messages.ArchiveData, error) {
									t.Error("source overlap reached extraction")
									return messages.ArchiveData{}, nil
								})
							if err == nil {
								t.Fatal("source overlap accepted with extractor")
							}
							for _, name := range []string{source, raw} {
								if _, err := os.Stat(name + ".sync.lock"); !os.IsNotExist(err) {
									t.Fatalf("source-side lock created before refusal: %v", err)
								}
							}
						}
						assertBundle(t, source, before)
						assertBundle(t, raw, decoyBefore)
					})
				}
			}
		}
	}
}

func TestArchiveWhitespacePositiveOpenAndMigration(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "current", true: "legacy"}[legacy], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "archive.db")
			createWhitespaceArchiveFixture(t, path, legacy)
			for _, raw := range []string{path + " ", path, path + "\t", path + "\u00a0"} {
				st, err := OpenExisting(context.Background(), raw)
				if err != nil {
					t.Fatal(err)
				}
				version, err := st.store.SchemaVersion(context.Background())
				if err != nil || version != schemaVersion || st.path != raw || st.store.Path() != path {
					t.Fatalf("opened wrong archive: version=%d path=%q display=%q err=%v", version, st.store.Path(), st.path, err)
				}
				st.Close()
				st, err = Open(context.Background(), raw)
				if err != nil {
					t.Fatal(err)
				}
				st.Close()
			}
			if _, err := os.Stat(path + " "); !os.IsNotExist(err) {
				t.Fatalf("literal whitespace archive unexpectedly created: %v", err)
			}
		})
	}
}

func TestSyncPreservesLiteralWhitespaceSource(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "source.db "), filepath.Join(root, "archive.db")
	db := createWhitespaceMessagesFixture(t, source, "WAL")
	defer db.Close()
	decoy := filepath.Join(root, "source.db")
	if err := os.WriteFile(decoy, []byte("synthetic unselected source"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, decoyBefore := captureBundle(t, source), captureBundle(t, decoy)
	result, err := Sync(context.Background(), target+" ", source, false)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(source)
	if err != nil || result.SourcePath != resolved || result.ArchivePath != target+" " || result.Messages != 1 {
		t.Fatalf("wrong source or display path: %#v, %v", result, err)
	}
	st, err := OpenExisting(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if got := scalar(t, st.store.DB(), `select count(*) from messages where guid='synthetic-source-message'`); got != 1 {
		t.Fatalf("literal source message count = %d", got)
	}
	assertBundle(t, source, before)
	assertBundle(t, decoy, decoyBefore)
}

func createWhitespaceMessagesFixture(t *testing.T, path, journal string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("pragma journal_mode=" + journal + `;
create table handle(id text, service text, uncanonicalized_id text);
create table chat(guid text, chat_identifier text, service_name text, display_name text, room_name text, is_archived integer);
create table chat_handle_join(chat_id integer, handle_id integer);
create table chat_message_join(chat_id integer, message_id integer);
create table message_attachment_join(message_id integer, attachment_id integer);
create table message(guid text, handle_id integer, date integer, service text, is_from_me integer, text text, attributedBody blob);
insert into message values('synthetic-source-message', 0, 1, 'iMessage', 0, 'synthetic text', x'');`)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	return db
}

func createWhitespaceArchiveFixture(t *testing.T, path string, legacy bool) {
	t.Helper()
	seed := filepath.Join(t.TempDir(), "seed.db")
	if legacy {
		db, err := sql.Open("sqlite", seed)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(`
create table messages(source_rowid integer primary key, guid text not null, handle_rowid integer not null default 0,
date integer not null default 0, service text, is_from_me integer not null default 0, text text, has_attachments integer not null default 0);
create table schema_migrations(version integer not null);
insert into schema_migrations values(1);`)
		closeErr := db.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("legacy fixture: %v, %v", err, closeErr)
		}
	} else {
		st, err := Open(context.Background(), seed)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.Close(); err != nil {
			t.Fatal(err)
		}
	}
	// Seed through an ordinary filename, then retain the literal decoy spelling.
	if err := os.Rename(seed, path); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustWorkingDirectory(t *testing.T) string {
	t.Helper()
	path, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return path
}
