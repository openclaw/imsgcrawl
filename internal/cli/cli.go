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
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printUsage(stdout)
		return nil
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

func (r *runtime) dispatch(args []string) error {
	switch args[0] {
	case "metadata":
		return r.print(controlManifest())
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
  imsgcrawl --version
`)
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
