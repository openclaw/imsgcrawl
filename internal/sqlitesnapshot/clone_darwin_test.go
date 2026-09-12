package sqlitesnapshot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestCloneIsPrivateAndIndependent(t *testing.T) {
	root := t.TempDir()
	source, dest := filepath.Join(root, "source"), filepath.Join(root, "clone")
	original := bytes.Repeat([]byte("synthetic snapshot\n"), 4096)
	if err := os.WriteFile(source, original, 0o444); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	cloned, err := cloneFile(source, dest, info)
	if err != nil {
		t.Fatal(err)
	}
	if !cloned {
		t.Skip("temporary filesystem does not support cloning")
	}
	private, err := os.Stat(dest)
	if err != nil || private.Mode().Perm() != 0o600 || os.SameFile(info, private) {
		t.Fatalf("clone must be a separate private file: %v, %v", private, err)
	}
	if err := os.WriteFile(dest, []byte("private recovery"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(source)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("clone write changed source: %v", err)
	}
	after, err := os.Stat(source)
	if err != nil || !sameState(info, after) {
		t.Fatalf("source metadata changed: %v", err)
	}
	if err := os.Chmod(source, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("later source write"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(dest)
	if err != nil || string(got) != "private recovery" {
		t.Fatalf("source write changed clone: %v", err)
	}
}

func TestCloneRejectsReplacedSourceAndExistingDestination(t *testing.T) {
	root := t.TempDir()
	source, dest := filepath.Join(root, "source"), filepath.Join(root, "clone")
	if err := os.WriteFile(source, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(source, source+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("replaced"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := cloneFile(source, dest, info); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("replaced source accepted: %v", err)
	}
	if err := os.WriteFile(dest, []byte("keep existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err = os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := copyFile(context.Background(), source, dest, info); err == nil {
		t.Fatal("existing destination accepted")
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != "keep existing" {
		t.Fatalf("existing destination changed: %v", err)
	}
}

func TestFlaggedSourceUsesPrivateByteCopy(t *testing.T) {
	root := t.TempDir()
	source, dest := filepath.Join(root, "source"), filepath.Join(root, "clone")
	data := []byte("synthetic flagged source")
	if err := os.WriteFile(source, data, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := unix.Chflags(source, unix.UF_NODUMP); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unix.Chflags(source, 0) })
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	if cloned, err := cloneFile(source, dest, info); cloned || err != nil {
		t.Fatalf("flagged source cloned: %v, %v", cloned, err)
	}
	digest, err := copyFile(context.Background(), source, dest, info)
	if err != nil || digest != sha256.Sum256(data) {
		t.Fatalf("byte-copy digest: %x, %v", digest, err)
	}
	var stat unix.Stat_t
	if err := unix.Stat(dest, &stat); err != nil || stat.Flags != 0 || stat.Mode&0o777 != 0o600 {
		t.Fatalf("byte-copy metadata: %+v, %v", stat, err)
	}
	if err := unix.Stat(source, &stat); err != nil || stat.Flags != unix.UF_NODUMP || stat.Mode&0o777 != 0o444 {
		t.Fatalf("source metadata changed: %+v, %v", stat, err)
	}
}
