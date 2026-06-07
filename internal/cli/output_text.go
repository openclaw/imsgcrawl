package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/openclaw/crawlkit/control"
	"github.com/openclaw/imsgcrawl/internal/archive"
)

func (r *runtime) print(v any) error {
	enc := json.NewEncoder(r.stdout)
	if r.json {
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	switch value := v.(type) {
	case control.Manifest:
		return printManifestText(r.stdout, value)
	case archive.SyncResult:
		return printSyncText(r.stdout, value)
	case statusOutput:
		return printStatusText(r.stdout, value)
	case chatListOutput:
		return printChatsText(r.stdout, value)
	case messageListOutput:
		return printMessagesText(r.stdout, value)
	case searchListOutput:
		return printSearchText(r.stdout, value)
	case contactExport:
		return printContactsText(r.stdout, value)
	default:
		return enc.Encode(v)
	}
}

func printManifestText(w io.Writer, value control.Manifest) error {
	if _, err := fmt.Fprintf(w, "imsgcrawl metadata\nid: %s\nname: %s\n", value.ID, value.DisplayName); err != nil {
		return err
	}
	if value.Description != "" {
		if _, err := fmt.Fprintf(w, "description: %s\n", value.Description); err != nil {
			return err
		}
	}
	if len(value.Capabilities) > 0 {
		if _, err := fmt.Fprintf(w, "capabilities: %s\n", strings.Join(value.Capabilities, ", ")); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, "commands:\n"); err != nil {
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, name := range []string{"metadata", "status", "sync", "chats", "messages", "search", "contact-export"} {
		command, ok := value.Commands[name]
		if !ok {
			continue
		}
		if _, err := fmt.Fprintf(tw, "  %s\t%s\n", name, strings.Join(command.Argv, " ")); err != nil {
			return err
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	_, err := io.WriteString(w, "json: add --json for the machine-readable manifest\n")
	return err
}

func printSyncText(w io.Writer, value archive.SyncResult) error {
	_, err := fmt.Fprintf(w, "sync complete\narchive_path: %s\nsource_path: %s\nhandles: %d\nchats: %d\nchat_messages: %d\nmessages: %d\nsynced_at: %s\n",
		value.ArchivePath, value.SourcePath, value.Handles, value.Chats, value.ChatMessages, value.Messages, value.SyncedAt)
	return err
}

func printStatusText(w io.Writer, value statusOutput) error {
	if _, err := fmt.Fprintf(w, "status: %s\nsummary: %s\n", value.State, value.Summary); err != nil {
		return err
	}
	if value.Source != nil {
		if _, err := fmt.Fprintf(w, "source_db_path: %s\nsource_handles: %d\nsource_chats: %d\nsource_messages: %d\n", value.Source.DatabasePath, value.Source.Handles, value.Source.Chats, value.Source.Messages); err != nil {
			return err
		}
	}
	if value.Archive != nil {
		if _, err := fmt.Fprintf(w, "archive_path: %s\narchive_handles: %d\narchive_chats: %d\narchive_messages: %d\n", value.Archive.ArchivePath, value.Archive.Handles, value.Archive.Chats, value.Archive.Messages); err != nil {
			return err
		}
	}
	for _, warning := range value.Warnings {
		if _, err := fmt.Fprintf(w, "warning: %s\n", warning); err != nil {
			return err
		}
	}
	for _, msg := range value.Errors {
		if _, err := fmt.Fprintf(w, "error: %s\n", msg); err != nil {
			return err
		}
	}
	return nil
}

func printChatsText(w io.Writer, value chatListOutput) error {
	if _, err := fmt.Fprintf(w, "Chats: showing %d of %d, newest first.\n", value.Returned, value.Total); err != nil {
		return err
	}
	if !value.Complete {
		if _, err := fmt.Fprintf(w, "More: imsgcrawl chats --limit %d\nAll: imsgcrawl chats --all\n", nextLimit(value.Limit, value.Total)); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, "Open: imsgcrawl messages --chat CHAT_ID\n\n"); err != nil {
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "chat_id\tmessages\tlatest\ttitle"); err != nil {
		return err
	}
	for _, item := range value.Items {
		if _, err := fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n", item.ChatID, item.MessageCount, formatAppleDate(item.LatestMessageDate), cleanCell(item.Title, 72)); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func printMessagesText(w io.Writer, value messageListOutput) error {
	if _, err := fmt.Fprintf(w, "Messages for chat %s: showing %d of %d, %s.\n", value.ChatID, value.Returned, value.Total, value.Order); err != nil {
		return err
	}
	if !value.Complete {
		if _, err := fmt.Fprintf(w, "More: imsgcrawl messages --chat %s --limit %d\nAll: imsgcrawl messages --chat %s --all\n", value.ChatID, nextLimit(value.Limit, value.Total), value.ChatID); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, "Search: imsgcrawl search QUERY\n\n"); err != nil {
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "message_id\tdate\tfrom\tservice\ttext"); err != nil {
		return err
	}
	for _, item := range value.Items {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", item.MessageID, formatAppleDate(item.Date), messageSide(item.FromMe), item.Service, cleanCell(item.Text, 120)); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func printSearchText(w io.Writer, value searchListOutput) error {
	if _, err := fmt.Fprintf(w, "Search %q: showing %d of %d.\n", value.Query, value.Returned, value.Total); err != nil {
		return err
	}
	if !value.Complete {
		if _, err := fmt.Fprintf(w, "More: imsgcrawl search --limit %d %s\nAll: imsgcrawl search --all %s\n", nextLimit(value.Limit, value.Total), strconv.Quote(value.Query), strconv.Quote(value.Query)); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, "Open: imsgcrawl messages --chat CHAT_ID\n\n"); err != nil {
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "message_id\tchat_id\tdate\tfrom\tservice\tsnippet"); err != nil {
		return err
	}
	for _, item := range value.Items {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", item.MessageID, item.ChatID, formatAppleDate(item.Date), messageSide(item.FromMe), item.Service, cleanCell(item.Snippet, 120)); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func printContactsText(w io.Writer, value contactExport) error {
	for _, contact := range value.Contacts {
		_, err := fmt.Fprintf(w, "%s\t%s\n", contact.DisplayName, strings.Join(contact.PhoneNumbers, ","))
		if err != nil {
			return err
		}
	}
	return nil
}

func nextLimit(limit int, total int64) int {
	if limit <= 0 {
		return int(total)
	}
	next := limit * 2
	if int64(next) > total {
		return int(total)
	}
	return next
}

func formatAppleDate(value int64) string {
	if value <= 0 {
		return "-"
	}
	epoch := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	return epoch.Add(time.Duration(value)).Local().Format("2006-01-02 15:04")
}

func messageSide(fromMe bool) string {
	if fromMe {
		return "me"
	}
	return "them"
}

func cleanCell(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= limit {
		if value == "" {
			return "-"
		}
		return value
	}
	if limit <= 1 {
		return value[:limit]
	}
	return value[:limit-1] + "..."
}
