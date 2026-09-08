//go:build unix

package archive

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

func lockSync(ctx context.Context, path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	// Retain the lock inode after unlocking so waiters and new arrivals cannot
	// lock different files. Never follow a caller-supplied lock-file symlink.
	fd, err := unix.Open(path+".sync.lock", unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open archive sync lock: %w", err)
	}
	file := os.NewFile(uintptr(fd), path+".sync.lock")
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, errors.New("archive sync lock must be a regular file")
	}
	closeFile := func() { _ = file.Close() }
	if err := flockContext(ctx, fd); err != nil {
		closeFile()
		return nil, err
	}
	// Never flock the database: on macOS that can interact with SQLite's fcntl
	// locks. Hardlink aliases also have different WAL names, so refuse writes.
	info, err = os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return closeFile, nil
	}
	if err != nil {
		closeFile()
		return nil, err
	}
	linked, err := archiveHardlinked(path, info)
	if err != nil || linked {
		closeFile()
		return nil, errors.New("sync requires an archive without hardlink aliases")
	}
	return closeFile, nil
}

func flockContext(ctx context.Context, fd int) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			return fmt.Errorf("lock archive sync: %w", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}
