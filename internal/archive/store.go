package archive

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/openclaw/crawlkit/store"
	"github.com/openclaw/imsgcrawl/internal/messages"
)

type Store struct {
	store *store.Store
	path  string
}

func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".imsgcrawl", "archive.db")
	}
	return filepath.Join(home, ".imsgcrawl", "archive.db")
}

func Exists(path string) bool {
	if path == "" {
		path = DefaultPath()
	}
	filename, err := archiveFilename(path)
	if err != nil {
		return false
	}
	info, err := os.Stat(filename)
	return err == nil && info.Mode().IsRegular()
}

func Open(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		path = DefaultPath()
	}
	sqlitePath, err := archiveFilename(path)
	if err != nil {
		return nil, err
	}
	if _, err := inspectArchive(ctx, sqlitePath, true); err != nil {
		return nil, err
	}
	st, err := store.Open(ctx, store.Options{Path: sqlitePath, Schema: schema})
	if err != nil {
		return nil, err
	}
	if err := migrate(ctx, st); err != nil {
		_ = st.Close()
		return nil, err
	}
	return &Store{store: st, path: path}, nil
}

func OpenExisting(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		path = DefaultPath()
	}
	sqlitePath, err := archiveFilename(path)
	if err != nil {
		return nil, err
	}
	version, err := inspectArchive(ctx, sqlitePath, false)
	if err != nil {
		return nil, err
	}
	if version < schemaVersion {
		return Open(ctx, path)
	}
	st, err := store.OpenReadOnly(ctx, sqlitePath)
	if err != nil {
		return nil, err
	}
	return &Store{store: st, path: path}, nil
}

func (s *Store) Close() error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.Close()
}

func Sync(ctx context.Context, archivePath, sourcePath string, restore bool) (SyncResult, error) {
	return syncArchive(ctx, archivePath, sourcePath, restore, messages.ExtractArchive)
}

func syncArchive(ctx context.Context, archivePath, sourcePath string, restore bool,
	extract func(context.Context, string) (messages.ArchiveData, error),
) (SyncResult, error) {
	if archivePath == "" {
		archivePath = DefaultPath()
	}
	if sourcePath == "" {
		sourcePath = messages.DefaultChatDBPath()
	}
	sqlitePath, err := archiveFilename(archivePath)
	if err != nil {
		return SyncResult{}, err
	}
	if err := rejectSourceOverlap(sqlitePath, sourcePath); err != nil {
		return SyncResult{}, err
	}
	lockPath, err := canonicalPath(sqlitePath)
	if err != nil {
		return SyncResult{}, err
	}
	unlock, err := lockSync(ctx, lockPath)
	if err != nil {
		return SyncResult{}, err
	}
	defer unlock()
	if _, err := inspectArchive(ctx, sqlitePath, true); err != nil {
		return SyncResult{}, err
	}
	data, err := extract(ctx, sourcePath)
	if err != nil {
		return SyncResult{}, err
	}
	if err := rejectSourceOverlap(sqlitePath, sourcePath); err != nil {
		return SyncResult{}, err
	}
	st, err := Open(ctx, sqlitePath)
	if err != nil {
		return SyncResult{}, err
	}
	defer func() { _ = st.Close() }()
	now := time.Now().UTC()
	if err := st.Import(ctx, data, now, restore); err != nil {
		return SyncResult{}, err
	}
	mode := "merge"
	if restore {
		mode = "restore"
	}
	return SyncResult{
		ArchivePath:      archivePath,
		SourcePath:       data.SourcePath,
		SourceBytes:      data.SourceBytes,
		SourceModifiedAt: data.SourceModifiedAt.Format(time.RFC3339),
		SyncedAt:         now.Format(time.RFC3339),
		Mode:             mode,
		Handles:          len(data.Handles),
		Chats:            len(data.Chats),
		Participants:     len(data.Participants),
		ChatMessages:     len(data.ChatMessages),
		Messages:         len(data.Messages),
	}, nil
}
