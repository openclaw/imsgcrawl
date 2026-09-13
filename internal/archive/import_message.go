package archive

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/openclaw/imsgcrawl/internal/messages"
)

func importMessage(ctx context.Context, tx *sql.Tx, message messages.Message, syncedAt time.Time, restore bool) error {
	if !restore {
		var err error
		message, err = hydrateUnavailableRevisionMetadata(ctx, tx, message)
		if err != nil {
			return err
		}
	}
	if (message.DateEdited > 0 || message.HasEdits || message.HasUnsentParts) && !message.TextAvailable {
		message.Text = ""
	}
	deletedAt, reason := messageTombstone(message, syncedAt)
	messageKey, messageIdentity := "guid", any(message.GUID)
	if strings.TrimSpace(message.GUID) == "" {
		messageKey, messageIdentity = "source_rowid", message.SourceRowID
	}
	existingAt, existingReason, err := existingTombstone(ctx, tx, "messages", messageKey, messageIdentity)
	if err != nil {
		return err
	}
	if existingAt != nil {
		deletedAt, reason = existingAt, existingReason
	}
	if _, err := tx.ExecContext(ctx, insertMessagesSQL, message.SourceRowID, message.GUID, message.HandleRowID,
		message.Date, message.Service, boolInt(message.IsFromMe), message.Text, boolInt(message.HasAttachments),
		message.DateEdited, message.DateRetracted, message.RevisionData, deletedAt, nullableReason(reason)); err != nil {
		return err
	}
	if message.SourceRowID > 0 {
		if _, err := tx.ExecContext(ctx, `delete from messages where guid = ? and source_rowid < 0`, message.GUID); err != nil {
			return err
		}
	}
	if err := appendMessageEvent(ctx, tx, message, syncedAt); err != nil {
		return err
	}
	if deletedAt != nil {
		if err := tombstoneMessageChildren(ctx, tx, message.SourceRowID, *deletedAt, reason); err != nil {
			return err
		}
	}
	return nil
}

func hydrateUnavailableRevisionMetadata(ctx context.Context, tx *sql.Tx, message messages.Message) (messages.Message, error) {
	if message.TextAvailable && message.DateEditedAvailable && message.DateRetractedAvailable && message.RevisionDataAvailable {
		return message, nil
	}
	var existingText string
	var dateEdited, dateRetracted int64
	var revisionData []byte
	err := tx.QueryRowContext(ctx, `select coalesce(text, ''), date_edited, date_retracted, revision_data
from messages where source_rowid = ?`, message.SourceRowID).Scan(&existingText, &dateEdited, &dateRetracted, &revisionData)
	if errors.Is(err, sql.ErrNoRows) {
		return message, nil
	}
	if err != nil {
		return message, err
	}
	if !message.DateEditedAvailable {
		message.DateEdited = dateEdited
	}
	if !message.DateRetractedAvailable {
		message.DateRetracted = dateRetracted
	}
	revisionDataHydrated := !message.RevisionDataAvailable
	if !message.RevisionDataAvailable {
		message.RevisionData = revisionData
	}
	if revisionDataHydrated {
		message.ApplyRevisionData()
	} else if message.DateEdited > 0 && !message.HasEdits && !message.HasUnsentParts {
		message.TextAvailable = false
	}
	if !message.TextAvailable && (message.DateEdited > 0 || message.HasEdits || message.HasUnsentParts) {
		message.Text = ""
		message.TextAvailable = true
	} else if !message.TextAvailable {
		message.Text = existingText
		message.TextAvailable = true
	}
	return message, nil
}
