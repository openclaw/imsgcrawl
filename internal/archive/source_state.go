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
	var storedPath string
	err := tx.QueryRowContext(ctx, `select value from sync_state
where key in ('source_identity', 'source_path')
order by key = 'source_identity' desc limit 1`).Scan(&storedPath)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
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
