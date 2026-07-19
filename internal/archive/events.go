package archive

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/openclaw/imsgcrawl/internal/messages"
)

type messageEventPayload struct {
	GUID           string `json:"guid"`
	HandleRowID    int64  `json:"handle_rowid"`
	Date           int64  `json:"date"`
	Service        string `json:"service,omitempty"`
	IsFromMe       bool   `json:"is_from_me"`
	Text           string `json:"text,omitempty"`
	HasAttachments bool   `json:"has_attachments,omitempty"`
	DateEdited     int64  `json:"date_edited,omitempty"`
	DateRetracted  int64  `json:"date_retracted,omitempty"`
	HasEdits       bool   `json:"has_edits,omitempty"`
	HasUnsentParts bool   `json:"has_unsent_parts,omitempty"`
	FullyUnsent    bool   `json:"fully_unsent,omitempty"`
	RevisionAt     int64  `json:"revision_at,omitempty"`
}

func appendMessageEvent(ctx context.Context, tx *sql.Tx, message messages.Message, observedAt time.Time) error {
	payload, err := json.Marshal(messageEventPayload{
		GUID: message.GUID, HandleRowID: message.HandleRowID,
		Date: message.Date, Service: message.Service, IsFromMe: message.IsFromMe, Text: message.Text,
		HasAttachments: message.HasAttachments, DateEdited: message.DateEdited, DateRetracted: message.DateRetracted,
		HasEdits: message.HasEdits, HasUnsentParts: message.HasUnsentParts,
		FullyUnsent: message.FullyUnsent, RevisionAt: message.RevisionAt,
	})
	if err != nil {
		return err
	}
	eventType, revisionAt := messageEventType(message)
	hash := sha256.New()
	_, _ = hash.Write([]byte("imsgcrawl.message-event.v1\x00" + eventType + "\x00" + stableMessageIdentity(message) + "\x00"))
	_, _ = hash.Write(payload)
	_, _ = hash.Write([]byte{0})
	revisionData := eventRevisionData(message)
	_, _ = hash.Write([]byte(message.RevisionIdentity))
	eventKey := hex.EncodeToString(hash.Sum(nil))
	_, err = tx.ExecContext(ctx, `insert or ignore into message_events(
event_key, message_guid, source_rowid, event_type, revision_at, payload_json, revision_data, observed_at
) values(?, ?, ?, ?, ?, ?, ?, ?)`, eventKey, message.GUID, message.SourceRowID, eventType,
		revisionAt, string(payload), revisionData, observedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func stableMessageIdentity(message messages.Message) string {
	if guid := strings.TrimSpace(message.GUID); guid != "" {
		return "guid:" + guid
	}
	return "rowid:" + strconv.FormatInt(message.SourceRowID, 10)
}

func appendDeletionEvent(ctx context.Context, tx *sql.Tx, guid string, sourceRowID int64, observedAt time.Time, reason string) error {
	message := messages.Message{SourceRowID: sourceRowID, GUID: guid}
	payload, err := json.Marshal(struct {
		GUID   string `json:"guid"`
		Reason string `json:"reason"`
	}{GUID: guid, Reason: reason})
	if err != nil {
		return err
	}
	sum := sha256.Sum256(append([]byte("imsgcrawl.message-event.v1\x00message_deleted\x00"), payload...))
	_, err = tx.ExecContext(ctx, `insert or ignore into message_events(
event_key, message_guid, source_rowid, event_type, revision_at, payload_json, observed_at
) values(?, ?, ?, 'message_deleted', 0, ?, ?)`, hex.EncodeToString(sum[:]), message.GUID,
		message.SourceRowID, string(payload), observedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func messageEventType(message messages.Message) (string, int64) {
	switch {
	case message.DateRetracted > 0 || message.FullyUnsent:
		return "message_unsent", max(message.DateRetracted, message.RevisionAt)
	case message.HasUnsentParts:
		return "message_partial_unsent", max(message.DateEdited, message.RevisionAt)
	case message.DateEdited > 0 || message.HasEdits:
		return "message_edited", max(message.DateEdited, message.RevisionAt)
	default:
		return "message", message.Date
	}
}

func eventRevisionData(message messages.Message) []byte {
	if message.DateEdited > 0 || message.DateRetracted > 0 || message.HasEdits || message.HasUnsentParts {
		return message.RevisionData
	}
	return nil
}
