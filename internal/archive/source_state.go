package archive

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/openclaw/imsgcrawl/internal/messages"
)

func validateMergeSource(ctx context.Context, tx *sql.Tx, incomingPath string) error {
	if strings.TrimSpace(incomingPath) == "" {
		return nil
	}
	var storedKey, storedPath string
	err := tx.QueryRowContext(ctx, `select key, value from sync_state
where key in ('source_identity', 'source_path')
order by key = 'source_identity' desc limit 1`).Scan(&storedKey, &storedPath)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	// Version-1 archives stored relative source paths without the working
	// directory needed to resolve them. Adopt the first post-upgrade source;
	// identity validation still protects the merge, and replaceSyncState writes
	// an absolute source_identity that makes subsequent checks strict.
	if storedKey == "source_path" && !filepath.IsAbs(storedPath) {
		incomingIdentity, err := normalizedSourceIdentity(incomingPath)
		if err != nil {
			return err
		}
		if legacyRelativeSourceMatches(incomingIdentity, storedPath) {
			return nil
		}
		return fmt.Errorf("archive belongs to a different Messages source than %q; run sync --restore to replace it", incomingPath)
	}
	storedIdentity, err := normalizedSourceIdentity(storedPath)
	if err != nil {
		return err
	}
	incomingIdentity, err := normalizedSourceIdentity(incomingPath)
	if err != nil {
		return err
	}
	if storedIdentity != incomingIdentity {
		return fmt.Errorf("archive belongs to a different Messages source than %q; run sync --restore to replace it", incomingPath)
	}
	return nil
}

func legacyRelativeSourceMatches(incomingPath, storedPath string) bool {
	storedPath = filepath.Clean(storedPath)
	if storedPath == "." || storedPath == ".." || strings.HasPrefix(storedPath, ".."+string(filepath.Separator)) {
		return false
	}
	incomingPath = filepath.Clean(incomingPath)
	return incomingPath == storedPath || strings.HasSuffix(incomingPath, string(filepath.Separator)+storedPath)
}

func normalizedSourceIdentity(path string) (string, error) {
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(absPath); err == nil {
		return resolved, nil
	}
	return absPath, nil
}

func replaceSyncState(ctx context.Context, tx *sql.Tx, data messages.ArchiveData, syncedAt time.Time) error {
	state := map[string]string{
		"last_sync_at":        syncedAt.UTC().Format(time.RFC3339),
		"source_bytes":        strconv.FormatInt(data.SourceBytes, 10),
		"source_modified_at":  data.SourceModifiedAt.UTC().Format(time.RFC3339),
		"source_extracted_at": data.ExtractedAt.UTC().Format(time.RFC3339),
	}
	if strings.TrimSpace(data.SourcePath) != "" {
		sourceIdentity, err := normalizedSourceIdentity(data.SourcePath)
		if err != nil {
			return err
		}
		state["source_path"] = data.SourcePath
		state["source_identity"] = sourceIdentity
	}
	for key, value := range state {
		if _, err := tx.ExecContext(ctx, upsertSyncStateSQL, key, value); err != nil {
			return err
		}
	}
	return nil
}
