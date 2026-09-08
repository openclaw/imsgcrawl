package archive

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/openclaw/crawlkit/store"
	"github.com/openclaw/imsgcrawl/internal/sqlitesnapshot"
)

func inspectArchive(ctx context.Context, path string, allowNew bool) (int, error) {
	// This preflight prevents accidental foreign-file selection. Callers must
	// keep the pathname stable through SQLite's subsequent writable open.
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) && allowNew {
		return 0, inspectArchiveSidecars(path, true)
	}
	if err != nil {
		return 0, err
	}
	if !info.Mode().IsRegular() {
		return 0, errors.New("archive must be a regular file")
	}
	if err := inspectArchiveSidecars(path, info.Size() == 0); err != nil {
		return 0, err
	}
	privatePath, cleanup, err := sqlitesnapshot.Copy(ctx, path)
	if err != nil {
		return 0, err
	}
	defer cleanup()
	db, err := store.OpenReadOnly(ctx, privatePath)
	if err != nil {
		return 0, err
	}
	defer db.Close()
	version, err := db.SchemaVersion(ctx)
	if err != nil {
		return 0, err
	}
	if version > schemaVersion {
		return 0, fmt.Errorf("archive schema version %d is newer than supported version %d", version, schemaVersion)
	}
	var tables int
	if err := db.DB().QueryRowContext(ctx, `select count(*) from sqlite_master where name not like 'sqlite_%'`).Scan(&tables); err != nil {
		return 0, err
	}
	if allowNew && tables == 0 {
		linked, err := archiveHardlinked(path, info)
		if err != nil {
			return 0, err
		}
		if linked {
			return 0, errors.New("cannot initialize a hardlinked empty database: its source WAL may belong to another filename")
		}
		return 0, nil
	}
	// Version-one archives may contain only this original table. The version
	// table alone is shared with other apps and does not establish ownership.
	var columns int
	if err := db.DB().QueryRowContext(ctx, `select count(*) from pragma_table_info('messages')
where name in ('source_rowid','guid','handle_rowid','date','service','is_from_me','text','has_attachments')`).Scan(&columns); err != nil {
		return 0, err
	}
	if columns != 8 {
		return 0, errors.New("database is not an imsgcrawl archive")
	}
	after, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	if !os.SameFile(info, after) {
		return 0, errors.New("archive changed identity during validation")
	}
	return version, nil
}

func inspectArchiveSidecars(path string, missing bool) error {
	base, err := canonicalPath(path)
	if err != nil {
		return err
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		info, err := os.Lstat(base + suffix)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if missing || !info.Mode().IsRegular() {
			return errors.New("archive has an unowned or non-regular SQLite sidecar")
		}
		linked, err := archiveHardlinked(base+suffix, info)
		if err != nil {
			return err
		}
		if linked {
			return errors.New("archive has a hardlinked SQLite sidecar")
		}
	}
	return nil
}

// canonicalPath resolves existing parents even when the final file is absent.
func canonicalPath(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return canonicalPathWithLinks(path, 0)
}

func canonicalPathWithLinks(path string, links int) (string, error) {
	if links > 255 {
		return "", errors.New("too many symbolic links in archive path")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	info, statErr := os.Lstat(path)
	if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		return canonicalPathWithLinks(target, links+1)
	}
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return "", statErr
	}
	parent := filepath.Dir(path)
	if parent == path {
		return "", err
	}
	resolved, err = canonicalPathWithLinks(parent, links)
	return filepath.Join(resolved, filepath.Base(path)), err
}

func rejectSourceOverlap(archivePath, sourcePath string) error {
	archivePath, err := canonicalPath(archivePath)
	if err != nil {
		return err
	}
	sourcePath, err = canonicalPath(sourcePath)
	if err != nil {
		return err
	}
	for _, destinationSuffix := range []string{"", "-wal", "-shm", "-journal", ".sync.lock"} {
		destination, err := canonicalPath(archivePath + destinationSuffix)
		if err != nil {
			return err
		}
		destInfo, destErr := os.Stat(destination)
		if destErr != nil && !errors.Is(destErr, os.ErrNotExist) {
			return destErr
		}
		for _, sourceSuffix := range []string{"", "-wal", "-shm", "-journal"} {
			source, err := canonicalPath(sourcePath + sourceSuffix)
			if err != nil {
				return err
			}
			sourceInfo, sourceErr := os.Stat(source)
			if sourceErr != nil && !errors.Is(sourceErr, os.ErrNotExist) {
				return sourceErr
			}
			if destination == source || (destInfo != nil && sourceInfo != nil && os.SameFile(destInfo, sourceInfo)) {
				return errors.New("archive output overlaps the Messages source or its sidecars")
			}
		}
	}
	return nil
}
