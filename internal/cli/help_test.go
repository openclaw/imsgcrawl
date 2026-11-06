package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestHelpIsCligDevShaped(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"no command", nil, "imsgcrawl reads local iMessage"},
		{"top help long", []string{"--help"}, "imsgcrawl help COMMAND"},
		{"top help short", []string{"-h"}, "Global flags:"},
		{"help command", []string{"help"}, "imsgcrawl help COMMAND"},
		{"help chats", []string{"help", "chats"}, "Default: 50"},
		{"help contacts export", []string{"help", "contacts", "export"}, "contacts export"},
		{"metadata help", []string{"metadata", "--help"}, "Print crawlkit control metadata"},
		{"sync help", []string{"sync", "--help"}, "Merge the Messages database into the local archive"},
		{"status help", []string{"status", "-h"}, "aggregate counts"},
		{"chats help", []string{"chats", "--help"}, "--all"},
		{"messages help", []string{"messages", "--help"}, "Default: 20"},
		{"search help", []string{"search", "--help"}, "Default: 20"},
		{"contacts help", []string{"contacts", "--help"}, "contacts export"},
		{"contacts export help", []string{"contacts", "export", "--help"}, "--json"},
		{"json contacts export help", []string{"--json", "contacts", "export", "--help"}, "--json"},
		{"chats help with invalid limit", []string{"chats", "--limit", "-1", "--help"}, "List archived chats."},
		{"messages help with invalid arguments", []string{"messages", "--chat", "", "--limit", "0", "--help"}, "List archived messages for one chat."},
		{"search help with invalid limit", []string{"search", "--limit", "0", "--help"}, "Search archived message text."},
		{"contacts export help with unexpected argument", []string{"contacts", "export", "unexpected", "--help"}, "Export phone contacts from the Messages source database."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if err := Run(context.Background(), tc.args, &stdout, &stderr); err != nil {
				t.Fatalf("Run(%v) error = %v stderr=%s", tc.args, err, stderr.String())
			}
			if !strings.Contains(stdout.String(), tc.want) {
				t.Fatalf("stdout missing %q:\n%s", tc.want, stdout.String())
			}
		})
	}
}

func TestVersion(t *testing.T) {
	if got := runOK(t, "--version"); strings.TrimSpace(got) != version {
		t.Fatalf("version = %q, want %q", got, version)
	}
}
