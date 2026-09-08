// Package sqlitesnapshot captures SQLite files without opening the source in SQLite.
package sqlitesnapshot

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/openclaw/crawlkit/store"
)

var suffixes = []string{"", "-wal", "-journal"}

// Copy returns an owned private copy. SHM is rebuilt only in that private directory.
func Copy(ctx context.Context, source string) (string, func(), error) {
	return copyWithVerifier(ctx, source, verify)
}

func copyWithVerifier(ctx context.Context, source string, verifyCopy func(context.Context, string) error) (string, func(), error) {
	resolved, err := filepath.EvalSymlinks(source)
	if err != nil {
		return "", func() {}, err
	}
	source = resolved
	root, err := os.MkdirTemp("", "imsgcrawl-snapshot-")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(root) }
	dest := filepath.Join(root, "snapshot.db")
	for attempt := 0; attempt < 5; attempt++ {
		if err := ctx.Err(); err != nil {
			cleanup()
			return "", func() {}, err
		}
		for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
			_ = os.Remove(dest + suffix)
		}
		stable, err := copyGeneration(ctx, source, dest)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanup()
			return "", func() {}, err
		}
		if stable && err == nil {
			if err := verifyCopy(ctx, dest); err == nil {
				return dest, cleanup, nil
			}
			if err := ctx.Err(); err != nil {
				cleanup()
				return "", func() {}, err
			}
		}
	}
	cleanup()
	if err := ctx.Err(); err != nil {
		return "", func() {}, err
	}
	return "", func() {}, fmt.Errorf("SQLite source changed or could not be verified after 5 snapshot attempts")
}

func fileStates(path string) (map[string]os.FileInfo, error) {
	result := make(map[string]os.FileInfo)
	for _, suffix := range suffixes {
		info, err := os.Stat(path + suffix)
		if suffix != "" && errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("SQLite source must be a regular file")
		}
		result[suffix] = info
	}
	return result, nil
}

func sameState(a, b os.FileInfo) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return os.SameFile(a, b) && a.Size() == b.Size() &&
		a.ModTime().Equal(b.ModTime()) && a.Mode() == b.Mode()
}

func copyGeneration(ctx context.Context, source, dest string) (bool, error) {
	before, err := fileStates(source)
	if err != nil {
		return false, err
	}
	digests := make(map[string][sha256.Size]byte)
	for _, suffix := range suffixes {
		if before[suffix] == nil {
			continue
		}
		out, err := os.OpenFile(dest+suffix, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return false, err
		}
		digest, err := readFile(ctx, source+suffix, before[suffix], out)
		closeErr := out.Close()
		if err != nil {
			return false, err
		}
		if closeErr != nil {
			return false, closeErr
		}
		digests[suffix] = digest
	}
	// Bracket both copies and verification reads with identity/metadata checks.
	// A SQLite integrity check alone cannot detect a valid but mixed generation.
	for _, suffix := range suffixes {
		if before[suffix] == nil {
			continue
		}
		digest, err := readFile(ctx, source+suffix, before[suffix], io.Discard)
		if err != nil {
			return false, err
		}
		if digest != digests[suffix] {
			return false, nil
		}
	}
	after, err := fileStates(source)
	if err != nil {
		return false, err
	}
	for _, suffix := range suffixes {
		if !sameState(before[suffix], after[suffix]) {
			return false, nil
		}
	}
	return true, nil
}

func readFile(ctx context.Context, path string, expected os.FileInfo, out io.Writer) ([sha256.Size]byte, error) {
	var result [sha256.Size]byte
	file, err := os.Open(path)
	if err != nil {
		return result, err
	}
	defer file.Close()
	actual, err := file.Stat()
	if err != nil {
		return result, err
	}
	if !sameState(expected, actual) {
		return result, os.ErrNotExist // retry a replaced or changing source
	}
	digest := sha256.New()
	reader := io.LimitReader(file, expected.Size())
	buffer := make([]byte, 64*1024)
	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		n, readErr := reader.Read(buffer)
		if n > 0 {
			if _, err := io.MultiWriter(out, digest).Write(buffer[:n]); err != nil {
				return result, err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return result, readErr
		}
	}
	copy(result[:], digest.Sum(nil))
	return result, nil
}

func verify(ctx context.Context, path string) error {
	// A copied rollback journal can be hot without the source's writer lock.
	// Recovery writes are allowed only in this owned private directory.
	db, err := store.Open(ctx, store.Options{Path: path})
	if err != nil {
		return err
	}
	defer db.Close()
	var result string
	if err := db.DB().QueryRowContext(ctx, "pragma quick_check").Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("SQLite snapshot integrity check failed")
	}
	return nil
}
