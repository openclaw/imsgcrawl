package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/openclaw/crawlkit/control"
	"github.com/openclaw/imsgcrawl/internal/messages"
)

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
	export := control.ContactExport{Contacts: contacts}
	if err := control.ValidateContactExport(export); err != nil {
		return err
	}
	return r.print(export)
}
