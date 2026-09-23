package archive

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSyncMalformedUTF16PreservesUneditedTextAndHidesKnownEdit(t *testing.T) {
	for name, raw := range map[string]string{
		"odd length":         "\xff\xfeA\x00B",
		"unpaired surrogate": "\xfe\xff\x00A\xd8\x3d",
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			source, target := filepath.Join(root, "source.db"), filepath.Join(root, "archive.db")
			db := createWhitespaceMessagesFixture(t, source, "WAL")
			if _, err := db.Exec(`alter table message add column date_edited integer default 0`); err != nil {
				t.Fatal(err)
			}
			if _, err := Sync(ctx, target, source, false); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`update message set text='', attributedBody=?`, revisionTypedStream(raw)); err != nil {
				t.Fatal(err)
			}
			for _, edited := range []bool{false, true} {
				if edited {
					if _, err := db.Exec(`update message set date_edited=2`); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := Sync(ctx, target, source, false); err != nil {
					t.Fatal(err)
				}
				st, err := OpenExisting(ctx, target)
				if err != nil {
					t.Fatal(err)
				}
				defer st.Close()
				var text string
				if err := st.store.DB().QueryRow(`select text from messages where guid='synthetic-source-message'`).Scan(&text); err != nil {
					t.Fatal(err)
				}
				wantText, wantMatches := "synthetic text", int64(1)
				if edited {
					wantText, wantMatches = "", 0
				}
				if text != wantText {
					t.Fatalf("edited=%v: archived text = %q, want %q", edited, text, wantText)
				}
				matches, err := st.CountSearch(ctx, "synthetic")
				if err != nil || matches != wantMatches {
					t.Fatalf("edited=%v: search matches = %d, %v; want %d", edited, matches, err, wantMatches)
				}
			}
		})
	}
}
