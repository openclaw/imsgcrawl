package archive

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/openclaw/imsgcrawl/internal/messages"
)

func validateMergeIdentities(ctx context.Context, tx *sql.Tx, data messages.ArchiveData) error {
	for _, message := range data.Messages {
		if err := validateStableRow(ctx, tx, "message", "messages", message.SourceRowID, "guid", message.GUID); err != nil {
			return err
		}
	}
	for _, chat := range data.Chats {
		if err := validateStableRow(ctx, tx, "chat", "chats", chat.SourceRowID, "guid", chat.GUID); err != nil {
			return err
		}
	}
	for _, handle := range data.Handles {
		var existingID, existingService string
		err := tx.QueryRowContext(ctx, `select handle, service from handles where source_rowid = ?`, handle.SourceRowID).Scan(&existingID, &existingService)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil && (existingID != handle.ID || existingService != handle.Service) {
			return identityConflict("handle", handle.SourceRowID,
				existingService+"/"+existingID, handle.Service+"/"+handle.ID)
		}
		if strings.TrimSpace(handle.ID) == "" {
			continue
		}
		var existingRowID int64
		err = tx.QueryRowContext(ctx, `select source_rowid from handles
where service = ? and handle = ? and source_rowid > 0 and source_rowid <> ? limit 1`,
			handle.Service, handle.ID, handle.SourceRowID).Scan(&existingRowID)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		return fmt.Errorf("source handle %q also exists at rowid %d instead of only rowid %d; run imsgcrawl sync --restore to replace the archive",
			handle.Service+"/"+handle.ID, existingRowID, handle.SourceRowID)
	}
	return validateIncomingIdentities(data)
}

func validateStableRow(ctx context.Context, tx *sql.Tx, entity, table string, sourceRowID int64, key, stableID string) error {
	var existingID string
	err := tx.QueryRowContext(ctx, `select `+key+` from `+table+` where source_rowid = ?`, sourceRowID).Scan(&existingID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && existingID != stableID {
		return identityConflict(entity, sourceRowID, existingID, stableID)
	}
	if strings.TrimSpace(stableID) == "" {
		return nil
	}
	var existingRowID int64
	err = tx.QueryRowContext(ctx, `select source_rowid from `+table+` where `+key+` = ? and source_rowid > 0 order by source_rowid limit 1`, stableID).Scan(&existingRowID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if existingRowID != sourceRowID {
		return fmt.Errorf("source %s %q moved from rowid %d to %d; run imsgcrawl sync --restore to replace the archive", entity, stableID, existingRowID, sourceRowID)
	}
	return nil
}

func validateIncomingIdentities(data messages.ArchiveData) error {
	messageRows := map[int64]string{}
	messageGUIDs := map[string]int64{}
	for _, message := range data.Messages {
		if err := rememberStableIdentity("message", message.SourceRowID, message.GUID, messageRows, messageGUIDs); err != nil {
			return err
		}
	}
	chatRows := map[int64]string{}
	chatGUIDs := map[string]int64{}
	for _, chat := range data.Chats {
		if err := rememberStableIdentity("chat", chat.SourceRowID, chat.GUID, chatRows, chatGUIDs); err != nil {
			return err
		}
	}
	handleRows := map[int64]string{}
	handleIdentities := map[string]int64{}
	for _, handle := range data.Handles {
		identity := handle.Service + "\x00" + handle.ID
		if existing, ok := handleRows[handle.SourceRowID]; ok && existing != identity {
			return identityConflict("handle", handle.SourceRowID, existing, identity)
		}
		handleRows[handle.SourceRowID] = identity
		if strings.TrimSpace(handle.ID) != "" {
			if existing, ok := handleIdentities[identity]; ok && existing != handle.SourceRowID {
				return fmt.Errorf("source handle %q appears at rowids %d and %d; run imsgcrawl sync --restore after repairing the source",
					handle.Service+"/"+handle.ID, existing, handle.SourceRowID)
			}
			handleIdentities[identity] = handle.SourceRowID
		}
	}
	return nil
}

func rememberStableIdentity(entity string, rowID int64, stableID string, rows map[int64]string, stableIDs map[string]int64) error {
	if existing, ok := rows[rowID]; ok && existing != stableID {
		return identityConflict(entity, rowID, existing, stableID)
	}
	rows[rowID] = stableID
	if strings.TrimSpace(stableID) == "" {
		return nil
	}
	if existing, ok := stableIDs[stableID]; ok && existing != rowID {
		return fmt.Errorf("source %s %q appears at rowids %d and %d; run imsgcrawl sync --restore after repairing the source", entity, stableID, existing, rowID)
	}
	stableIDs[stableID] = rowID
	return nil
}

func identityConflict(entity string, rowID int64, existingID, incomingID string) error {
	return fmt.Errorf("source %s rowid %d changed identity from %q to %q; run imsgcrawl sync --restore to replace the archive", entity, rowID, existingID, incomingID)
}
