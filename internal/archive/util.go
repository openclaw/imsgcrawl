package archive

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	ckstore "github.com/openclaw/crawlkit/store"
)

func countTable(ctx context.Context, db *sql.DB, table string) (int64, error) {
	var count int64
	if err := db.QueryRowContext(ctx, `select count(*) from `+table).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func parseID(value, label string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%s id must be a positive integer", label)
	}
	return id, nil
}

func ftsQuery(query string) string {
	value, err := ckstore.FTS5Terms(query, "")
	if err != nil {
		return ckstore.FTS5Phrase(query)
	}
	return value
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}
