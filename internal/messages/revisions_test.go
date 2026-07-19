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

func TestApplyRevisionDataReconstructsCurrentText(t *testing.T) {
	tests := []struct {
		name          string
		text          string
		textIsCurrent bool
		root          map[string]any
		wantText      string
		wantAvailable bool
	}{
		{
			name: "latest edit body",
			text: "original",
			root: map[string]any{
				"otr": map[string]any{"0": map[string]any{"lo": int64(0), "le": int64(8)}},
				"ec": map[string]any{"0": []any{
					map[string]any{"d": int64(1), "t": makeStreamtypedAttributedBody("first edit")},
					map[string]any{"d": int64(2), "t": makeStreamtypedAttributedBody("current edit")},
				}},
			},
			wantText:      "current edit",
			wantAvailable: true,
		},
		{
			name: "partial unsend removes part",
			text: "keep withdrawn stay",
			root: map[string]any{
				"otr": map[string]any{
					"0": map[string]any{"lo": int64(0), "le": int64(4)},
					"1": map[string]any{"lo": int64(4), "le": int64(10)},
					"2": map[string]any{"lo": int64(14), "le": int64(5)},
				},
				"rp": []any{int64(1)},
			},
			wantText:      "keep stay",
			wantAvailable: true,
		},
		{
			name: "malformed unsent part is ignored",
			text: "keep withdrawn",
			root: map[string]any{
				"otr": map[string]any{
					"0": map[string]any{"lo": int64(0), "le": int64(4)},
					"1": map[string]any{"lo": int64(-1), "le": int64(99)},
				},
				"ec": map[string]any{"1": []any{map[string]any{"d": int64(1), "t": []byte("invalid")}}},
				"rp": []any{int64(1)},
			},
			wantText:      "keep",
			wantAvailable: true,
		},
		{
			name: "unchanged part keeps original offset",
			text: "xtail",
			root: map[string]any{
				"otr": map[string]any{
					"0": map[string]any{"lo": int64(0), "le": int64(1)},
					"1": map[string]any{"lo": int64(1), "le": int64(4)},
				},
				"ec": map[string]any{"0": []any{map[string]any{"d": int64(1), "t": makeStreamtypedAttributedBody("LONGER")}}},
			},
			wantText:      "LONGERtail",
			wantAvailable: true,
		},
		{
			name:          "current attributed body uses adjusted offset",
			text:          "LONGERtail",
			textIsCurrent: true,
			root: map[string]any{
				"otr": map[string]any{
					"0": map[string]any{"lo": int64(0), "le": int64(1)},
					"1": map[string]any{"lo": int64(1), "le": int64(4)},
				},
				"ec": map[string]any{"0": []any{map[string]any{"d": int64(1), "t": makeStreamtypedAttributedBody("LONGER")}}},
			},
			wantText:      "LONGERtail",
			wantAvailable: true,
		},
		{
			name:          "current attributed body omits unsent part",
			text:          "keep stay",
			textIsCurrent: true,
			root: map[string]any{
				"otr": map[string]any{
					"0": map[string]any{"lo": int64(0), "le": int64(4)},
					"1": map[string]any{"lo": int64(4), "le": int64(10)},
					"2": map[string]any{"lo": int64(14), "le": int64(5)},
				},
				"rp": []any{int64(1)},
			},
			wantText:      "keep stay",
			wantAvailable: true,
		},
		{
			name: "equal length edits use original baseline",
			text: "ABCD",
			root: map[string]any{
				"otr": map[string]any{
					"0": map[string]any{"lo": int64(0), "le": int64(1)},
					"1": map[string]any{"lo": int64(1), "le": int64(1)},
					"2": map[string]any{"lo": int64(2), "le": int64(2)},
				},
				"ec": map[string]any{
					"0": []any{map[string]any{"d": int64(1), "t": makeStreamtypedAttributedBody("AB")}},
					"2": []any{map[string]any{"d": int64(1), "t": makeStreamtypedAttributedBody("D")}},
				},
			},
			wantText:      "ABBD",
			wantAvailable: true,
		},
		{
			name: "utf16 part ranges",
			text: "A😀B removed",
			root: map[string]any{
				"otr": map[string]any{
					"0": map[string]any{"lo": int64(0), "le": int64(4)},
					"1": map[string]any{"lo": int64(4), "le": int64(8)},
				},
				"rp": []any{int64(1)},
			},
			wantText:      "A😀B",
			wantAvailable: true,
		},
		{
			name: "undecodable edit body",
			text: "do not index fallback",
			root: map[string]any{
				"otr": map[string]any{"0": map[string]any{"lo": int64(0), "le": int64(21)}},
				"ec":  map[string]any{"0": []any{map[string]any{"d": int64(1), "t": []byte("invalid")}}},
			},
			wantText:      "do not index fallback",
			wantAvailable: false,
		},
		{
			name: "negative original offset",
			text: "safe fallback",
			root: map[string]any{
				"otr": map[string]any{"0": map[string]any{"lo": int64(-1), "le": int64(1)}},
				"ec":  map[string]any{"0": []any{map[string]any{"d": int64(1), "t": makeStreamtypedAttributedBody("edit")}}},
			},
			wantText:      "safe fallback",
			wantAvailable: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := plist.Marshal(test.root, plist.BinaryFormat)
			if err != nil {
				t.Fatal(err)
			}
			message := Message{Text: test.text, TextAvailable: true, TextIsCurrent: test.textIsCurrent, RevisionData: data}
			message.ApplyRevisionData()
			if message.Text != test.wantText || message.TextAvailable != test.wantAvailable {
				t.Fatalf("current text = %q available=%v, want %q available=%v", message.Text, message.TextAvailable, test.wantText, test.wantAvailable)
			}
		})
	}
}
