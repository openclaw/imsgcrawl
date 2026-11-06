package cli

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

func TestTextOutputWidthUsesWideColumnsEnv(t *testing.T) {
	t.Setenv("COLUMNS", "180")
	var out bytes.Buffer
	if got := textOutputWidth(&out); got != 180 {
		t.Fatalf("text output width = %d, want 180", got)
	}
}

func TestRenderTextTableDoesNotPadFinalColumn(t *testing.T) {
	var out bytes.Buffer
	if err := renderTextTable(&out, []textColumn{
		{header: "left", width: 8},
		{header: "text", width: 20, wrap: true},
	}, [][]string{{"x", "short"}}); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "left      text\nx         short\n"; got != want {
		t.Fatalf("table = %q, want %q", got, want)
	}
}

func TestRenderTextTablePreservesWrappedContents(t *testing.T) {
	const sender = "long.synthetic.sender@example.test"
	for _, test := range []struct {
		name    string
		columns []textColumn
		row     []string
		want    []string
	}{
		{
			name:    "search context",
			columns: searchTextColumns(88),
			row: []string{
				"2026-06-07 09:10", sender,
				"group with second@example.test", "A complete synthetic result.",
			},
			want: []string{
				"date from conversation text",
				"2026-06-07 09:10 long.synthetic.s group with A complete synthetic result.",
				"ender@example.te second@example.test",
				"st",
			},
		},
		{
			name:    "message sender",
			columns: messageTextColumns(88),
			row:     []string{"2026-06-07 09:10", sender, "A complete synthetic message."},
			want: []string{
				"date from text",
				"2026-06-07 09:10 long.synthetic.sender@examp A complete synthetic message.",
				"le.test",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := renderTextTable(&out, test.columns, [][]string{test.row}); err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
			for i, line := range lines {
				lines[i] = strings.Join(strings.Fields(line), " ")
			}
			if !slices.Equal(lines, test.want) {
				t.Fatalf("rendered lines = %q, want %q", lines, test.want)
			}
		})
	}
}
