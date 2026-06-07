package cli

import (
	"bytes"
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
	err := renderTextTable(&out, []textColumn{
		{header: "left", width: 8},
		{header: "text", width: 20, wrap: true},
	}, [][]string{{"x", "short"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n") {
		if strings.HasSuffix(line, " ") {
			t.Fatalf("line has trailing padding: %q", line)
		}
	}
}
