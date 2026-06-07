package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/openclaw/imsgcrawl/internal/archive"
	"github.com/openclaw/imsgcrawl/internal/messages"
)

type cliError struct {
	code int
	err  error
}

func (e *cliError) Error() string { return e.err.Error() }
func (e *cliError) Unwrap() error { return e.err }

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, context.Canceled) {
		return 1
	}
	var codeErr *cliError
	if errors.As(err, &codeErr) {
		return codeErr.code
	}
	return 1
}

type runtime struct {
	ctx         context.Context
	stdout      io.Writer
	stderr      io.Writer
	json        bool
	dbPath      string
	archivePath string
}

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	jsonOut, args := pullJSONFlag(args)
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printUsage(stdout)
		return nil
	}
	if args[0] == "help" {
		if len(args) == 1 {
			printUsage(stdout)
			return nil
		}
		return printCommandUsage(stdout, args[1:])
	}
	global := flag.NewFlagSet("imsgcrawl", flag.ContinueOnError)
	global.SetOutput(io.Discard)
	dbPath := global.String("db", messages.DefaultChatDBPath(), "")
	archivePath := global.String("archive", archive.DefaultPath(), "")
	versionFlag := global.Bool("version", false, "")
	if err := global.Parse(args); err != nil {
		return usageErr(err)
	}
	if *versionFlag {
		_, _ = io.WriteString(stdout, version+"\n")
		return nil
	}
	rest := global.Args()
	if len(rest) == 0 || rest[0] == "help" || rest[0] == "--help" || rest[0] == "-h" {
		if len(rest) > 1 && rest[0] == "help" {
			return printCommandUsage(stdout, rest[1:])
		}
		printUsage(stdout)
		return nil
	}
	if rest[0] == "version" {
		_, _ = io.WriteString(stdout, version+"\n")
		return nil
	}
	r := &runtime{ctx: ctx, stdout: stdout, stderr: stderr, json: jsonOut, dbPath: *dbPath, archivePath: *archivePath}
	return r.dispatch(rest)
}

func pullJSONFlag(args []string) (bool, []string) {
	out := make([]string, 0, len(args))
	jsonOut := false
	for _, arg := range args {
		if arg == "--json" {
			jsonOut = true
			continue
		}
		out = append(out, arg)
	}
	return jsonOut, out
}

func flagPassed(fs *flag.FlagSet, name string) bool {
	passed := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			passed = true
		}
	})
	return passed
}

func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" || arg == "-help" {
			return true
		}
	}
	return false
}

func (r *runtime) dispatch(args []string) error {
	switch args[0] {
	case "metadata":
		return r.runMetadata(args[1:])
	case "sync":
		return r.runSync(args[1:])
	case "status":
		return r.runStatus(args[1:])
	case "chats":
		return r.runChats(args[1:])
	case "messages":
		return r.runMessages(args[1:])
	case "search":
		return r.runSearch(args[1:])
	case "contacts":
		return r.runContacts(args[1:])
	default:
		return usageErr(fmt.Errorf("unknown command %q", args[0]))
	}
}

func (r *runtime) runContacts(args []string) error {
	if hasHelpFlag(args) {
		return printCommandUsage(r.stdout, []string{"contacts"})
	}
	if len(args) == 0 {
		return usageErr(errors.New("usage: imsgcrawl contacts export"))
	}
	switch args[0] {
	case "export":
		return r.runContactsExport(args[1:])
	default:
		return usageErr(fmt.Errorf("unknown contacts command %q", args[0]))
	}
}

type contactExport struct {
	Contacts []messages.ExportedContact `json:"contacts"`
}

func (r *runtime) runContactsExport(args []string) error {
	if hasHelpFlag(args) {
		return printCommandUsage(r.stdout, []string{"contacts", "export"})
	}
	fs := flag.NewFlagSet("imsgcrawl contacts export", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return usageErr(err)
	}
	if fs.NArg() != 0 {
		return usageErr(errors.New("contacts export takes no arguments"))
	}
	contacts, err := messages.ExportContacts(r.ctx, r.dbPath)
	if err != nil {
		return err
	}
	return r.print(contactExport{Contacts: contacts})
}

func (r *runtime) runMetadata(args []string) error {
	if hasHelpFlag(args) {
		return printCommandUsage(r.stdout, []string{"metadata"})
	}
	if len(args) != 0 {
		return usageErr(errors.New("metadata takes no arguments"))
	}
	return r.print(controlManifest())
}

func (r *runtime) print(v any) error {
	enc := json.NewEncoder(r.stdout)
	if r.json {
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	switch value := v.(type) {
	case statusOutput:
		if value.Source != nil {
			if _, err := fmt.Fprintf(r.stdout, "source_db_path: %s\nsource_handles: %d\nsource_chats: %d\nsource_messages: %d\n", value.Source.DatabasePath, value.Source.Handles, value.Source.Chats, value.Source.Messages); err != nil {
				return err
			}
		}
		if value.Archive != nil {
			if _, err := fmt.Fprintf(r.stdout, "archive_path: %s\narchive_handles: %d\narchive_chats: %d\narchive_messages: %d\n", value.Archive.ArchivePath, value.Archive.Handles, value.Archive.Chats, value.Archive.Messages); err != nil {
				return err
			}
		}
		for _, warning := range value.Warnings {
			if _, err := fmt.Fprintf(r.stdout, "warning: %s\n", warning); err != nil {
				return err
			}
		}
		for _, msg := range value.Errors {
			if _, err := fmt.Fprintf(r.stdout, "error: %s\n", msg); err != nil {
				return err
			}
		}
		return nil
	case contactExport:
		for _, contact := range value.Contacts {
			_, err := fmt.Fprintf(r.stdout, "%s\t%s\n", contact.DisplayName, strings.Join(contact.PhoneNumbers, ","))
			if err != nil {
				return err
			}
		}
		return nil
	default:
		return enc.Encode(v)
	}
}

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
  --json       Print JSON output where supported.
  --db PATH    Messages source database path.
  --archive PATH
              Local imsgcrawl archive path.

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
  --limit N   Maximum chats to print. Default: all.
  --all       Print all chats. This is also the default.
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

func usageErr(err error) error {
	return &cliError{code: 2, err: err}
}

func defaultBaseDir() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".imsgcrawl")
	}
	return ".imsgcrawl"
}
