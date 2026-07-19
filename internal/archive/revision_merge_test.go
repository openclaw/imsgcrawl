package archive

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"howett.net/plist"
)

func TestMergePreservesUnavailableRevisionMetadata(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "archive.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	modern := fixtureArchiveData()
	modern.Messages = modern.Messages[:1]
	modern.ChatMessages = modern.ChatMessages[:1]
	summaryData, err := plist.Marshal(map[string]any{
		"otr": map[string]any{"0": map[string]any{}},
		"ec":  map[string]any{"0": []any{map[string]any{"d": int64(123)}}},
	}, plist.BinaryFormat)
	if err != nil {
		t.Fatal(err)
	}
	modern.Messages[0].DateEdited = 123
	modern.Messages[0].RevisionData = summaryData
	modern.Messages[0].ApplyRevisionData()
	if err := st.Import(ctx, modern, now, false); err != nil {
		t.Fatal(err)
	}
	legacy := fixtureArchiveData()
	legacy.Messages = legacy.Messages[:1]
	legacy.ChatMessages = legacy.ChatMessages[:1]
	legacy.Messages[0].DateEdited = 0
	legacy.Messages[0].RevisionData = nil
	legacy.Messages[0].DateEditedAvailable = false
	legacy.Messages[0].DateRetractedAvailable = false
	legacy.Messages[0].RevisionDataAvailable = false
	if err := st.Import(ctx, legacy, now.Add(time.Minute), false); err != nil {
		t.Fatal(err)
	}
	var dateEdited int64
	var revisionData []byte
	if err := st.store.DB().QueryRow(`select date_edited, revision_data from messages where guid='message-one'`).Scan(&dateEdited, &revisionData); err != nil {
		t.Fatal(err)
	}
	if dateEdited != 123 || len(revisionData) == 0 {
		t.Fatalf("merged revision metadata = %d, %q", dateEdited, revisionData)
	}
	if got := scalar(t, st.store.DB(), `select count(*) from message_events`); got != 1 {
		t.Fatalf("legacy merge duplicated revision event: %d", got)
	}
	if err := st.Import(ctx, legacy, now.Add(2*time.Minute), true); err != nil {
		t.Fatal(err)
	}
	if err := st.store.DB().QueryRow(`select date_edited, revision_data from messages where guid='message-one'`).Scan(&dateEdited, &revisionData); err != nil {
		t.Fatal(err)
	}
	if dateEdited != 0 || len(revisionData) != 0 {
		t.Fatalf("restored revision metadata = %d, %q", dateEdited, revisionData)
	}
}
