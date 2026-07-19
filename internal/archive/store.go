package archive

import (
	"context"
	"fmt"
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
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func Open(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		path = DefaultPath()
	}
	st, err := store.Open(ctx, store.Options{Path: path, Schema: schema})
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
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	st, err := store.OpenReadOnly(ctx, path)
	if err != nil {
		return nil, err
	}
	version, err := st.SchemaVersion(ctx)
	if err != nil {
		_ = st.Close()
		return nil, err
	}
	if version < schemaVersion {
		_ = st.Close()
		return nil, fmt.Errorf("archive schema version %d needs migration; run imsgcrawl sync", version)
	}
	if version > schemaVersion {
		_ = st.Close()
		return nil, fmt.Errorf("archive schema version %d is newer than supported version %d", version, schemaVersion)
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
	data, err := messages.ExtractArchive(ctx, sourcePath)
	if err != nil {
		return SyncResult{}, err
	}
	st, err := Open(ctx, archivePath)
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
		ArchivePath:      st.path,
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
