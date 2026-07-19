package messages

import (
	"testing"

	"howett.net/plist"
)

func TestParseMessageSummaryInfo(t *testing.T) {
	tests := []struct {
		name         string
		root         map[string]any
		want         revisionState
		wantIdentity bool
	}{
		{
			name: "unrelated metadata",
			root: map[string]any{"translation": "translated text"},
		},
		{
			name: "edited part",
			root: map[string]any{
				"otr": map[string]any{"0": map[string]any{}},
				"ec":  map[string]any{"0": []any{map[string]any{"d": int64(804340730)}}},
			},
			want:         revisionState{HasEdits: true, RevisionAt: 804340730000000000},
			wantIdentity: true,
		},
		{
			name: "edited part with real timestamp",
			root: map[string]any{
				"otr": map[string]any{"0": map[string]any{}},
				"ec":  map[string]any{"0": []any{map[string]any{"d": 804340730.125}}},
			},
			want:         revisionState{HasEdits: true, RevisionAt: 804340730125000000},
			wantIdentity: true,
		},
		{
			name: "partial unsend",
			root: map[string]any{
				"otr": map[string]any{"0": map[string]any{}, "1": map[string]any{}},
				"rp":  []any{int64(1)},
			},
			want:         revisionState{HasUnsentParts: true},
			wantIdentity: true,
		},
		{
			name: "full unsend",
			root: map[string]any{
				"otr": map[string]any{"0": map[string]any{}, "1": map[string]any{}},
				"rp":  []any{int64(0), int64(1)},
			},
			want:         revisionState{HasUnsentParts: true, FullyUnsent: true},
			wantIdentity: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := plist.Marshal(test.root, plist.BinaryFormat)
			if err != nil {
				t.Fatal(err)
			}
			got := parseMessageSummaryInfo(data)
			if (got.Identity != "") != test.wantIdentity {
				t.Fatalf("revision identity presence = %v, want %v", got.Identity != "", test.wantIdentity)
			}
			got.Identity = ""
			if got != test.want {
				t.Fatalf("revision state = %#v, want %#v", got, test.want)
			}
		})
	}
	if got := parseMessageSummaryInfo([]byte("not a plist")); got != (revisionState{}) {
		t.Fatalf("malformed summary state = %#v", got)
	}
	base := map[string]any{
		"otr":         map[string]any{"0": map[string]any{}},
		"ec":          map[string]any{"0": []any{map[string]any{"d": int64(804340730)}}},
		"translation": "first",
	}
	changed := map[string]any{
		"otr":         base["otr"],
		"ec":          base["ec"],
		"translation": "second",
	}
	firstData, err := plist.Marshal(base, plist.BinaryFormat)
	if err != nil {
		t.Fatal(err)
	}
	secondData, err := plist.Marshal(changed, plist.BinaryFormat)
	if err != nil {
		t.Fatal(err)
	}
	first := parseMessageSummaryInfo(firstData)
	second := parseMessageSummaryInfo(secondData)
	if first.Identity == "" || first.Identity != second.Identity {
		t.Fatalf("unrelated metadata changed revision identity: %q != %q", first.Identity, second.Identity)
	}
}
