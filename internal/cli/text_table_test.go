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

func TestSearchColumnsWrapContextWithoutEllipsis(t *testing.T) {
	var out bytes.Buffer
	err := renderTextTable(&out, searchTextColumns(88), [][]string{{
		"2026-06-07 09:10",
		"georgepcloud@icloud.com",
		"group with georgepcloud@icloud.com, michaelpalmer123@icloud.com",
		"And banana",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "...") {
		t.Fatalf("search table truncated context:\n%s", out.String())
	}
}

func TestMessageColumnsWrapSenderWithoutServiceOrEllipsis(t *testing.T) {
	var out bytes.Buffer
	err := renderTextTable(&out, messageTextColumns(88), [][]string{{
		"2026-06-07 09:10",
		"michaelpalmer123@icloud.com",
		"Sure. This message should keep sender identity visible.",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "...") {
		t.Fatalf("message table truncated sender:\n%s", out.String())
	}
	if strings.Contains(out.String(), "service") || strings.Contains(out.String(), "iMessage") {
		t.Fatalf("message table kept service column:\n%s", out.String())
	}
}
