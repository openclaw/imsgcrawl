package archive

import (
	"context"
	"database/sql"
	"time"

	"github.com/openclaw/imsgcrawl/internal/messages"
)

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
			if err := importMessage(ctx, tx, message, syncedAt, restore); err != nil {
				return err
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
