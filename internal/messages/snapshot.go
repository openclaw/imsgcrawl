package messages

import (
	"context"
	"os"
	"path/filepath"

	"github.com/openclaw/imsgcrawl/internal/sqlitesnapshot"
)

type Snapshot struct {
	SourcePath string
	Path       string
	root       string
}

func SnapshotPath(path string) (Snapshot, error) {
	return SnapshotPathContext(context.Background(), path)
}

func SnapshotPathContext(ctx context.Context, path string) (Snapshot, error) {
	if path == "" {
		path = DefaultChatDBPath()
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return Snapshot{}, err
	}
	path, err = filepath.EvalSymlinks(absPath)
	if err != nil {
		return Snapshot{}, err
	}
	result, _, err := sqlitesnapshot.Copy(ctx, path)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{SourcePath: path, Path: result, root: filepath.Dir(result)}, nil
}

func (s Snapshot) Close() error {
	if s.root == "" {
		return nil
	}
	return os.RemoveAll(s.root)
}
