package cli

import (
	"fmt"
	"io"
	"strings"
)

func printUsage(w io.Writer) {
	_, _ = fmt.Fprint(w, `imsgcrawl reads local iMessage Messages data.

Usage:
  imsgcrawl [--json] [--db PATH] metadata
  imsgcrawl [--json] [--db PATH] [--archive PATH] sync
  imsgcrawl [--json] [--db PATH] [--archive PATH] status
  imsgcrawl [--json] [--archive PATH] chats [--limit N|--all]
  imsgcrawl [--json] [--archive PATH] messages --chat ID [--limit N|--all] [--asc]
  imsgcrawl [--json] [--archive PATH] search [--limit N|--all] QUERY
  imsgcrawl [--json] [--db PATH] contacts export
  imsgcrawl help COMMAND
  imsgcrawl --version

Global flags:
  --json       Print machine-readable JSON output.
  --db PATH    Messages source database path.
  --archive PATH
              Local imsgcrawl archive path.

Output:
  Default output is compact text for humans and agents.
  Use --json for stable machine parsing.
  Use --all only when you explicitly want complete local output.

Help:
  imsgcrawl help chats
  imsgcrawl chats --help
`)
}

func printCommandUsage(w io.Writer, args []string) error {
	topic := strings.Join(args, " ")
	switch topic {
	case "metadata":
		_, _ = fmt.Fprint(w, `Usage:
  imsgcrawl [--json] [--db PATH] metadata

Print crawlkit control metadata.
`)
	case "sync":
		_, _ = fmt.Fprint(w, `Usage:
  imsgcrawl [--json] [--db PATH] [--archive PATH] sync

Refresh the local imsgcrawl archive from the Messages database.
`)
	case "status":
		_, _ = fmt.Fprint(w, `Usage:
  imsgcrawl [--json] [--db PATH] [--archive PATH] status

Report source/archive readability and aggregate counts.
`)
	case "chats":
		_, _ = fmt.Fprint(w, `Usage:
  imsgcrawl [--json] [--archive PATH] chats [--limit N|--all]

List archived chats.

Flags:
  --limit N   Maximum chats to print. Default: 50.
  --all       Print all chats. Use explicitly for complete local output.
`)
	case "messages":
		_, _ = fmt.Fprint(w, `Usage:
  imsgcrawl [--json] [--archive PATH] messages --chat ID [--limit N|--all] [--asc]

List archived messages for one chat.

Flags:
  --chat ID   Chat ID from imsgcrawl chats.
  --limit N   Maximum messages to print. Default: 50.
  --all       Print all messages for the chat.
  --asc       Show oldest messages first.
`)
	case "search":
		_, _ = fmt.Fprint(w, `Usage:
  imsgcrawl [--json] [--archive PATH] search [--limit N|--all] QUERY

Search archived message text.

Flags:
  --limit N   Maximum search results. Default: 20.
  --all       Print all matching search results.
`)
	case "contacts", "contacts export":
		_, _ = fmt.Fprint(w, `Usage:
  imsgcrawl [--json] [--db PATH] contacts export

Export phone contacts from the Messages source database.
`)
	default:
		return usageErr(fmt.Errorf("unknown help topic %q", topic))
	}
	return nil
}
