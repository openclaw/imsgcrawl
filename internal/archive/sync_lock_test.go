//go:build unix

package archive

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/openclaw/imsgcrawl/internal/messages"
)

func TestSyncLockAcrossProcesses(t *testing.T) {
	if path := os.Getenv("IMSGCRAWL_TEST_LOCK"); path != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		unlock, err := lockSync(ctx, path)
		if unlock != nil {
			unlock()
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("child lock = %v", err)
		}
		return
	}
	path := filepath.Join(t.TempDir(), "archive.db")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	unlock, err := lockSync(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	cmd := exec.Command(os.Args[0], "-test.run=^TestSyncLockAcrossProcesses$")
	cmd.Env = append(os.Environ(), "IMSGCRAWL_TEST_LOCK="+path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("child: %v\n%s", err, output)
	}
}

func TestSyncRejectsHardlinkedArchiveBeforeExtraction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.db")
	st, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(path, filepath.Join(filepath.Dir(path), "alias.db")); err != nil {
		t.Fatal(err)
	}
	_, err = syncArchive(context.Background(), path, filepath.Join(t.TempDir(), "source.db"), false,
		func(context.Context, string) (messages.ArchiveData, error) {
			t.Error("hardlinked archive reached extraction")
			return messages.ArchiveData{}, nil
		})
	if err == nil {
		t.Fatal("hardlinked writable archive accepted")
	}
}

func TestSyncWaitsForLockBeforeArchiveInspection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.db")
	unlock, err := lockSync(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	// Invalid bytes make any inspection before the owner's lock is released
	// fail deterministically, without depending on WAL writer timing.
	if err := os.WriteFile(path, []byte("synthetic in-progress state"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := captureBundle(t, path)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = syncArchive(ctx, path, filepath.Join(t.TempDir(), "source.db"), false,
		func(context.Context, string) (messages.ArchiveData, error) {
			t.Error("archive inspection and extraction must wait for the lock")
			return messages.ArchiveData{}, nil
		})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("sync inspected the archive before waiting: %v", err)
	}
	assertBundle(t, path, before)
}

func TestSyncSerializesExtractionThroughImport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.db")
	source := filepath.Join(t.TempDir(), "source.db")
	firstStarted, firstRelease := make(chan struct{}), make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		_, err := syncArchive(context.Background(), path, source, false,
			func(context.Context, string) (messages.ArchiveData, error) {
				close(firstStarted)
				<-firstRelease
				data := fixtureArchiveData()
				data.SourcePath = source
				return data, nil
			})
		firstDone <- err
	}()
	<-firstStarted
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := syncArchive(ctx, path, source, false, func(context.Context, string) (messages.ArchiveData, error) {
		t.Error("second extraction ran before first import")
		return messages.ArchiveData{}, nil
	})
	close(firstRelease)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second sync = %v", err)
	}
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	_, err = syncArchive(context.Background(), path, source, false, func(context.Context, string) (messages.ArchiveData, error) {
		return messages.ArchiveData{}, errors.New("synthetic extraction failure")
	})
	if err == nil {
		t.Fatal("extraction error lost")
	}
	unlock, err := lockSync(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
}

func TestSyncSerializesDanglingArchiveSymlink(t *testing.T) {
	root := t.TempDir()
	target, alias := filepath.Join(root, "archive.db"), filepath.Join(root, "alias.db")
	if err := os.Symlink(filepath.Base(target), alias); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "synthetic-source.db")
	started, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := syncArchive(context.Background(), alias, source, false,
			func(context.Context, string) (messages.ArchiveData, error) {
				close(started)
				<-release
				return fixtureArchiveData(), nil
			})
		done <- err
	}()
	select {
	case <-started:
	case err := <-done:
		t.Fatalf("first sync did not reach extraction: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := syncArchive(ctx, target, source, false, func(context.Context, string) (messages.ArchiveData, error) {
		t.Error("target-name extraction bypassed dangling-symlink lock")
		return messages.ArchiveData{}, errors.New("unexpected extraction")
	})
	close(release)
	firstErr := <-done
	if !errors.Is(err, context.DeadlineExceeded) || firstErr != nil {
		t.Fatalf("second=%v first=%v", err, firstErr)
	}
}
