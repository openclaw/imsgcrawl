package archive

import (
	"context"
	"testing"
	"time"

	"howett.net/plist"
)

func TestMergePreservesUnavailableRevisionMetadata(t *testing.T) {
	ctx := context.Background()
	st := openTestArchive(t, ctx)
	defer func() { _ = st.Close() }()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	modern := fixtureArchiveData()
	modern.Messages = modern.Messages[:1]
	modern.ChatMessages = modern.ChatMessages[:1]
	summaryData, err := plist.Marshal(map[string]any{
		"otr": map[string]any{"0": map[string]any{"lo": int64(0), "le": int64(13)}},
		"ec": map[string]any{"0": []any{map[string]any{
			"d": int64(123), "t": revisionTypedStream("original text"),
		}}},
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

func TestMergeClearsUnavailableRevisionText(t *testing.T) {
	for _, tc := range []struct {
		name, currentText, incomingText string
		root                            map[string]any
		partialUnsend                   bool
		searchTerms                     []string
	}{
		{
			name: "undecodable edit", currentText: "known current text", incomingText: "withdrawn fallback",
			root: map[string]any{
				"otr": map[string]any{"0": map[string]any{"lo": int64(0), "le": int64(18)}},
				"ec":  map[string]any{"0": []any{map[string]any{"d": int64(123), "t": []byte("invalid")}}},
			},
			searchTerms: []string{"withdrawn", "known"},
		},
		{
			name: "unreconstructable partial unsend", currentText: "keep withdrawn", incomingText: "keep withdrawn",
			root: map[string]any{
				"otr": map[string]any{
					"0": map[string]any{"lo": int64(0), "le": int64(4)},
					"1": map[string]any{"lo": int64(4), "le": int64(10)},
				},
				"ec": map[string]any{"0": []any{map[string]any{"d": int64(123), "t": []byte("invalid")}}},
				"rp": []any{int64(1)},
			},
			partialUnsend: true, searchTerms: []string{"withdrawn", "keep"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			st := openTestArchive(t, ctx)
			defer func() { _ = st.Close() }()
			now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
			current := fixtureArchiveData()
			current.Messages = current.Messages[:1]
			current.ChatMessages = current.ChatMessages[:1]
			current.Messages[0].Text = tc.currentText
			if err := st.Import(ctx, current, now, false); err != nil {
				t.Fatal(err)
			}
			incoming := fixtureArchiveData()
			incoming.Messages = incoming.Messages[:1]
			incoming.ChatMessages = incoming.ChatMessages[:1]
			incoming.Messages[0].Text = tc.incomingText
			var err error
			incoming.Messages[0].RevisionData, err = plist.Marshal(tc.root, plist.BinaryFormat)
			if err != nil {
				t.Fatal(err)
			}
			incoming.Messages[0].ApplyRevisionData()
			if incoming.Messages[0].TextAvailable || incoming.Messages[0].HasUnsentParts != tc.partialUnsend {
				t.Fatalf("invalid revision state = %#v", incoming.Messages[0])
			}
			if err := st.Import(ctx, incoming, now.Add(time.Minute), false); err != nil {
				t.Fatal(err)
			}
			var text string
			if err := st.store.DB().QueryRow("select text from messages where guid='message-one'").Scan(&text); err != nil {
				t.Fatal(err)
			}
			if text != "" {
				t.Fatalf("unsafe revision text = %q", text)
			}
			for _, term := range tc.searchTerms {
				var hits int
				if err := st.store.DB().QueryRow("select count(*) from messages_fts where messages_fts match ?", term).Scan(&hits); err != nil {
					t.Fatal(err)
				}
				if hits != 0 {
					t.Fatalf("unsafe revision search %q: %d hits", term, hits)
				}
			}
		})
	}
}

func TestMergeDoesNotReapplyAvailableRevisionData(t *testing.T) {
	ctx := context.Background()
	st := openTestArchive(t, ctx)
	defer func() { _ = st.Close() }()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	incoming := fixtureArchiveData()
	incoming.Messages = incoming.Messages[:1]
	incoming.ChatMessages = incoming.ChatMessages[:1]
	incoming.Messages[0].Text = "xtail"
	incoming.Messages[0].DateEditedAvailable = false
	var err error
	incoming.Messages[0].RevisionData, err = plist.Marshal(map[string]any{
		"otr": map[string]any{
			"0": map[string]any{"lo": int64(0), "le": int64(1)},
			"1": map[string]any{"lo": int64(1), "le": int64(4)},
		},
		"ec": map[string]any{"0": []any{map[string]any{"d": int64(123), "t": revisionTypedStream("LONGER")}}},
	}, plist.BinaryFormat)
	if err != nil {
		t.Fatal(err)
	}
	incoming.Messages[0].ApplyRevisionData()
	for index := 0; index < 2; index++ {
		if err := st.Import(ctx, incoming, now.Add(time.Duration(index)*time.Minute), false); err != nil {
			t.Fatal(err)
		}
	}
	var text string
	if err := st.store.DB().QueryRow(`select text from messages where guid='message-one'`).Scan(&text); err != nil {
		t.Fatal(err)
	}
	if text != "LONGERtail" {
		t.Fatalf("repeated merge current text = %q", text)
	}
	if got := scalar(t, st.store.DB(), `select count(*) from message_events`); got != 1 {
		t.Fatalf("repeated merge event count = %d", got)
	}
}

func revisionTypedStream(text string) []byte {
	out := []byte("\x04\x0bstreamtyped\x81\xe8\x03\x84\x01@\x84\x84\x84\x12NSAttributedString\x00\x84\x84\x08NSObject\x00\x85\x92\x84\x84\x84\x08NSString\x01\x94\x84\x01+")
	if len(text) <= 0x7f {
		out = append(out, byte(len(text)))
	} else {
		out = append(out, 0x81, byte(len(text)), byte(len(text)>>8))
	}
	out = append(out, text...)
	return append(out, 0x86)
}
