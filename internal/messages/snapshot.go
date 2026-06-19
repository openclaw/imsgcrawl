package messages

import (
	"os"

	"github.com/openclaw/crawlkit/cache"
)

type Snapshot struct {
	SourcePath string
	Path       string
	root       string
}

func SnapshotPath(path string) (Snapshot, error) {
	if path == "" {
		path = DefaultChatDBPath()
	}
	root, err := os.MkdirTemp("", "imsgcrawl-snapshot-*")
	if err != nil {
		return Snapshot{}, err
	}
	result, err := cache.SnapshotSQLite(cache.SQLiteSnapshotOptions{SourcePath: path, DestinationDir: root})
	if err != nil {
		_ = os.RemoveAll(root)
		return Snapshot{}, err
	}
	return Snapshot{SourcePath: path, Path: result.Path, root: root}, nil
}

func (s Snapshot) Close() error {
	if s.root == "" {
		return nil
	}
	return os.RemoveAll(s.root)
}
