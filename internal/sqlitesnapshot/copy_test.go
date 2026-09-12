package sqlitesnapshot

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyDigestDetectsSameSizeSourceChange(t *testing.T) {
	root := t.TempDir()
	source, dest := filepath.Join(root, "source"), filepath.Join(root, "copy")
	original := []byte("original")
	if err := os.WriteFile(source, original, 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := copyFile(context.Background(), source, dest, info)
	if err != nil || digest != sha256.Sum256(original) {
		t.Fatalf("private copy digest: %x, %v", digest, err)
	}
	if err := os.WriteFile(source, []byte("modified"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(source, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(source)
	if err != nil || !sameState(info, after) {
		t.Fatalf("fixture metadata must match: %v", err)
	}
	verification, err := readFile(context.Background(), source, info, io.Discard)
	if err != nil || verification == digest {
		t.Fatalf("verification missed source change: %v", err)
	}
}

func TestCopyFileCancellationCreatesNothing(t *testing.T) {
	root := t.TempDir()
	source, dest := filepath.Join(root, "source"), filepath.Join(root, "copy")
	if err := os.WriteFile(source, []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := copyFile(ctx, source, dest, info); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("destination created after cancellation: %v", err)
	}
}
