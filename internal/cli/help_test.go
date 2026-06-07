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
		{"top help long", []string{"--help"}, "imsgcrawl help COMMAND"},
		{"top help short", []string{"-h"}, "Global flags:"},
		{"help command", []string{"help"}, "imsgcrawl help COMMAND"},
		{"help chats", []string{"help", "chats"}, "Default: all"},
		{"help contacts export", []string{"help", "contacts", "export"}, "contacts export"},
		{"metadata help", []string{"metadata", "--help"}, "Print crawlkit control metadata"},
		{"sync help", []string{"sync", "--help"}, "Refresh the local imsgcrawl archive"},
		{"status help", []string{"status", "-h"}, "aggregate counts"},
		{"chats help", []string{"chats", "--help"}, "--all"},
		{"messages help", []string{"messages", "--help"}, "Default: 50"},
		{"search help", []string{"search", "--help"}, "Default: 20"},
		{"contacts help", []string{"contacts", "--help"}, "contacts export"},
		{"contacts export help", []string{"contacts", "export", "--help"}, "--json"},
		{"json contacts export help", []string{"--json", "contacts", "export", "--help"}, "--json"},
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

func TestHelpIgnoresOtherCommandArguments(t *testing.T) {
	for _, args := range [][]string{
		{"chats", "--limit", "-1", "--help"},
		{"messages", "--chat", "", "--limit", "0", "--help"},
		{"search", "--limit", "0", "--help"},
		{"contacts", "export", "unexpected", "--help"},
	} {
		var stdout, stderr bytes.Buffer
		if err := Run(context.Background(), args, &stdout, &stderr); err != nil {
			t.Fatalf("Run(%v) error = %v stderr=%s", args, err, stderr.String())
		}
		if stdout.Len() == 0 {
			t.Fatalf("Run(%v) printed no help", args)
		}
	}
}
