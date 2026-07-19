package archive

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/openclaw/imsgcrawl/internal/messages"
)

const (
	deletionExplicitFeed = "explicit-delete-feed"
	deletionSourceUnsent = "source-unsent"
)

var unixEpoch = time.Unix(0, 0).UTC()

func (s *Store) Import(ctx context.Context, data messages.ArchiveData, syncedAt time.Time, restore bool) error {
	return s.store.WithTx(ctx, func(tx *sql.Tx) error {
		if restore {
			if err := validateIncomingIdentities(data); err != nil {
				return err
			}
			for _, table := range []string{"messages_fts", "message_events", "chat_messages", "chat_participants", "messages", "chats", "handles", "sync_state"} {
				if _, err := tx.ExecContext(ctx, "delete from "+table); err != nil {
					return err
				}
			}
		} else {
			if err := validateMergeSource(ctx, tx, data.SourcePath); err != nil {
				return err
			}
			if err := validateMergeIdentities(ctx, tx, data); err != nil {
				return err
			}
		}
		for _, handle := range data.Handles {
			if _, err := tx.ExecContext(ctx, insertHandlesSQL, handle.SourceRowID, handle.ID, handle.Service, handle.UncanonicalizedID); err != nil {
				return err
			}
		}
		for _, chat := range data.Chats {
			deletedAt, reason, err := existingTombstone(ctx, tx, "chats", "guid", chat.GUID)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, insertChatsSQL, chat.SourceRowID, chat.GUID, chat.ChatIdentifier,
				chat.ServiceName, chat.DisplayName, chat.RoomName, boolInt(chat.IsArchived), deletedAt, nullableReason(reason)); err != nil {
				return err
			}
			if chat.SourceRowID > 0 {
				if _, err := tx.ExecContext(ctx, `delete from chats where guid = ? and source_rowid < 0`, chat.GUID); err != nil {
					return err
				}
			}
		}
		for _, participant := range data.Participants {
			if _, err := tx.ExecContext(ctx, insertChatParticipantsSQL, participant.ChatRowID, participant.HandleRowID); err != nil {
				return err
			}
		}
		for _, link := range data.ChatMessages {
			if _, err := tx.ExecContext(ctx, insertChatMessagesSQL, link.ChatRowID, link.MessageRowID); err != nil {
				return err
			}
		}
		for _, message := range data.Messages {
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
		}
		liveMessages := liveMessageGUIDs(data.Messages)
		for _, guid := range data.DeletedMessages {
			if liveMessages[guid] {
				continue
			}
			if err := tombstoneMessageGUID(ctx, tx, guid, syncedAt); err != nil {
				return err
			}
		}
		liveChats := liveChatGUIDs(data.Chats)
		for _, guid := range data.DeletedChats {
			if liveChats[guid] {
				continue
			}
			if err := tombstoneChatGUID(ctx, tx, guid, syncedAt); err != nil {
				return err
			}
		}
		if err := reconcileSubordinateTombstones(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "delete from messages_fts"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `insert into messages_fts(source_rowid, text)
select source_rowid, coalesce(text, '') from messages where deleted_at is null`); err != nil {
			return err
		}
		return replaceSyncState(ctx, tx, data, syncedAt)
	})
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

func liveMessageGUIDs(rows []messages.Message) map[string]bool {
	out := make(map[string]bool, len(rows))
	for _, message := range rows {
		guid := strings.TrimSpace(message.GUID)
		if guid != "" && !messageIsFullyUnsent(message) {
			out[guid] = true
		}
	}
	return out
}

func liveChatGUIDs(rows []messages.Chat) map[string]bool {
	out := make(map[string]bool, len(rows))
	for _, chat := range rows {
		if guid := strings.TrimSpace(chat.GUID); guid != "" {
			out[guid] = true
		}
	}
	return out
}

func existingTombstone(ctx context.Context, tx *sql.Tx, table, key string, value any) (*string, string, error) {
	var deletedAt string
	var reason sql.NullString
	err := tx.QueryRowContext(ctx, `select deleted_at, deletion_reason from `+table+
		` where `+key+` = ? and deleted_at is not null order by source_rowid > 0 desc limit 1`, value).Scan(&deletedAt, &reason)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(reason.String) == "" {
		reason.String = "retained-tombstone"
	}
	return &deletedAt, reason.String, nil
}

func nullableReason(reason string) any {
	if reason == "" {
		return nil
	}
	return reason
}

func reconcileSubordinateTombstones(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `update chat_participants
set deleted_at=coalesce(deleted_at, (select deleted_at from chats where chats.source_rowid = chat_participants.chat_rowid)),
deletion_reason=coalesce(deletion_reason, 'parent-chat-' || coalesce(
  nullif(trim((select deletion_reason from chats where chats.source_rowid = chat_participants.chat_rowid)), ''),
  'tombstoned'
))
where exists(select 1 from chats where chats.source_rowid = chat_participants.chat_rowid and chats.deleted_at is not null)`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `update chat_messages
set deleted_at=coalesce(deleted_at, (select deleted_at from chats where chats.source_rowid = chat_messages.chat_rowid)),
deletion_reason=coalesce(deletion_reason, 'parent-chat-' || coalesce(
  nullif(trim((select deletion_reason from chats where chats.source_rowid = chat_messages.chat_rowid)), ''),
  'tombstoned'
))
where exists(select 1 from chats where chats.source_rowid = chat_messages.chat_rowid and chats.deleted_at is not null)`); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `update chat_messages
set deleted_at=coalesce(deleted_at, (select deleted_at from messages where messages.source_rowid = chat_messages.message_rowid)),
deletion_reason=coalesce(deletion_reason, 'parent-message-' || coalesce(
  nullif(trim((select deletion_reason from messages where messages.source_rowid = chat_messages.message_rowid)), ''),
  'tombstoned'
))
where exists(select 1 from messages where messages.source_rowid = chat_messages.message_rowid and messages.deleted_at is not null)`)
	return err
}

func messageTombstone(message messages.Message, syncedAt time.Time) (*string, string) {
	if !messageIsFullyUnsent(message) {
		return nil, ""
	}
	value := appleTime(message.DateRetracted)
	if value.IsZero() {
		value = syncedAt.UTC()
	}
	formatted := value.Format(time.RFC3339Nano)
	return &formatted, deletionSourceUnsent
}

func messageIsFullyUnsent(message messages.Message) bool {
	return message.FullyUnsent || (message.DateRetracted > 0 && !message.HasUnsentParts)
}

func appleTime(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	epoch := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	if value < 1_000_000_000_000 {
		return epoch.Add(time.Duration(value) * time.Second)
	}
	return epoch.Add(time.Duration(value))
}

func tombstoneMessageGUID(ctx context.Context, tx *sql.Tx, guid string, observedAt time.Time) error {
	guid = strings.TrimSpace(guid)
	if guid == "" {
		return nil
	}
	deletedAt := observedAt.UTC().Format(time.RFC3339Nano)
	var rowID int64
	err := tx.QueryRowContext(ctx, `select source_rowid from messages where guid = ? order by source_rowid > 0 desc limit 1`, guid).Scan(&rowID)
	if errors.Is(err, sql.ErrNoRows) {
		rowID, err = allocateSyntheticRowID(ctx, tx, "message", "messages", guid)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `insert into messages(source_rowid, guid, deleted_at, deletion_reason)
values(?, ?, ?, ?)`, rowID, guid, deletedAt, deletionExplicitFeed); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `update messages set deleted_at=coalesce(deleted_at, ?),
deletion_reason=coalesce(deletion_reason, ?) where guid = ?`, deletedAt, deletionExplicitFeed, guid); err != nil {
		return err
	}
	if err := tombstoneMessageChildren(ctx, tx, rowID, deletedAt, deletionExplicitFeed); err != nil {
		return err
	}
	return appendDeletionEvent(ctx, tx, guid, rowID, observedAt, deletionExplicitFeed)
}

func tombstoneMessageChildren(ctx context.Context, tx *sql.Tx, rowID int64, deletedAt, reason string) error {
	_, err := tx.ExecContext(ctx, `update chat_messages set deleted_at=coalesce(deleted_at, ?),
deletion_reason=coalesce(deletion_reason, ?) where message_rowid = ?`, deletedAt, "parent-message-"+reason, rowID)
	return err
}

func tombstoneChatGUID(ctx context.Context, tx *sql.Tx, guid string, observedAt time.Time) error {
	guid = strings.TrimSpace(guid)
	if guid == "" {
		return nil
	}
	deletedAt := observedAt.UTC().Format(time.RFC3339Nano)
	var rowID int64
	err := tx.QueryRowContext(ctx, `select source_rowid from chats where guid = ? order by source_rowid > 0 desc limit 1`, guid).Scan(&rowID)
	if errors.Is(err, sql.ErrNoRows) {
		rowID, err = allocateSyntheticRowID(ctx, tx, "chat", "chats", guid)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `insert into chats(source_rowid, guid, deleted_at, deletion_reason)
values(?, ?, ?, ?)`, rowID, guid, deletedAt, deletionExplicitFeed); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `update chats set deleted_at=coalesce(deleted_at, ?),
deletion_reason=coalesce(deletion_reason, ?) where guid = ?`, deletedAt, deletionExplicitFeed, guid); err != nil {
		return err
	}
	for _, table := range []string{"chat_participants", "chat_messages"} {
		if _, err := tx.ExecContext(ctx, `update `+table+` set deleted_at=coalesce(deleted_at, ?),
deletion_reason=coalesce(deletion_reason, ?) where chat_rowid = ?`, deletedAt, "parent-chat-"+deletionExplicitFeed, rowID); err != nil {
			return err
		}
	}
	return nil
}

func allocateSyntheticRowID(ctx context.Context, tx *sql.Tx, kind, table, guid string) (int64, error) {
	for attempt := 0; attempt < 1024; attempt++ {
		rowID := syntheticRowID(kind, guid, attempt)
		var existingGUID string
		err := tx.QueryRowContext(ctx, `select guid from `+table+` where source_rowid = ?`, rowID).Scan(&existingGUID)
		if errors.Is(err, sql.ErrNoRows) || existingGUID == guid {
			return rowID, nil
		}
		if err != nil {
			return 0, err
		}
	}
	return 0, fmt.Errorf("allocate synthetic %s row for %q: collision limit exceeded", kind, guid)
}

func syntheticRowID(kind, guid string, attempt int) int64 {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d", kind, guid, attempt)))
	value := int64(binary.BigEndian.Uint64(sum[:8]) & ((1 << 63) - 1))
	if value == 0 {
		value = 1
	}
	return -value
}
