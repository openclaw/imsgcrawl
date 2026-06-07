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
	if _, err := fmt.Fprintf(w, "%s (%s)\n", value.DisplayName, value.ID); err != nil {
		return err
	}
	if value.Description != "" {
		if _, err := fmt.Fprintf(w, "%s\n", value.Description); err != nil {
			return err
		}
	}
	if len(value.Capabilities) > 0 {
		if _, err := fmt.Fprintf(w, "\nCapabilities: %s\n", strings.Join(value.Capabilities, ", ")); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, "\nAgent-facing commands:\n"); err != nil {
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
	_, err := io.WriteString(w, "\nMachine output: add --json to print the structured manifest.\n")
	return err
}

func printSyncText(w io.Writer, value archive.SyncResult) error {
	_, err := fmt.Fprintf(w, "Sync complete\n\nMessages source:\n  Database: %s\n  Modified: %s\n  Size: %d bytes\n\nLocal archive:\n  Database: %s\n  Synced: %s\n\nArchived rows:\n  Handles: %d\n  Chats: %d\n  Participants: %d\n  Chat-message links: %d\n  Messages: %d\n",
		value.SourcePath, emptyDash(value.SourceModifiedAt), value.SourceBytes, value.ArchivePath, value.SyncedAt, value.Handles, value.Chats, value.Participants, value.ChatMessages, value.Messages)
	return err
}

func printStatusText(w io.Writer, value statusOutput) error {
	if _, err := fmt.Fprintf(w, "Status: %s\n%s\n", value.State, value.Summary); err != nil {
		return err
	}
	if value.Source != nil {
		if _, err := fmt.Fprintf(w, "\nMessages source:\n  Database: %s\n  Handles: %d\n  Chats: %d\n  Messages: %d\n", value.Source.DatabasePath, value.Source.Handles, value.Source.Chats, value.Source.Messages); err != nil {
			return err
		}
	}
	if value.Archive != nil {
		if _, err := fmt.Fprintf(w, "\nLocal archive:\n  Database: %s\n  Last sync: %s\n  Handles: %d\n  Chats: %d\n  Participants: %d\n  Chat-message links: %d\n  Messages: %d\n", value.Archive.ArchivePath, emptyDash(value.Archive.LastSyncAt), value.Archive.Handles, value.Archive.Chats, value.Archive.Participants, value.Archive.ChatMessages, value.Archive.Messages); err != nil {
			return err
		}
	}
	if len(value.Warnings) > 0 {
		if _, err := io.WriteString(w, "\nWarnings:\n"); err != nil {
			return err
		}
		for _, warning := range value.Warnings {
			if _, err := fmt.Fprintf(w, "  - %s\n", warning); err != nil {
				return err
			}
		}
	}
	if len(value.Errors) > 0 {
		if _, err := io.WriteString(w, "\nErrors:\n"); err != nil {
			return err
		}
		for _, msg := range value.Errors {
			if _, err := fmt.Fprintf(w, "  - %s\n", msg); err != nil {
				return err
			}
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
	if _, err := fmt.Fprintln(tw, "chat\tkind\tmsgs\tlatest\tconversation"); err != nil {
		return err
	}
	for _, item := range value.Items {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\n", item.ChatID, item.Kind, item.MessageCount, formatAppleDate(item.LatestMessageDate), cleanCell(chatConversation(item), 96)); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func printMessagesText(w io.Writer, value messageListOutput) error {
	conversation := "chat " + value.ChatID
	if value.Chat != nil {
		conversation = chatConversation(*value.Chat)
	}
	if _, err := fmt.Fprintf(w, "Messages in %s (chat %s): showing %d of %d, %s.\n", conversation, value.ChatID, value.Returned, value.Total, value.Order); err != nil {
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
	if _, err := fmt.Fprintf(w, "%-16s  %-22s  %s\n", "date", "from", "service"); err != nil {
		return err
	}
	for _, item := range value.Items {
		service := item.Service
		if item.HasAttachments {
			service = strings.TrimSpace(service + " attachment(s)")
		}
		if _, err := fmt.Fprintf(w, "%-16s  %-22s  %s\n", formatAppleDate(item.Date), cleanCell(senderName(item.FromMe, item.SenderLabel), 22), emptyDash(service)); err != nil {
			return err
		}
		if err := writeIndentedBody(w, item.Text); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}
	return nil
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
	if _, err := fmt.Fprintln(tw, "#\tchat\tdate\tfrom\tconversation"); err != nil {
		return err
	}
	for i, item := range value.Items {
		if _, err := fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n", i+1, emptyDash(item.ChatID), formatAppleDate(item.Date), senderName(item.FromMe, item.SenderLabel), cleanCell(searchConversation(item), 72)); err != nil {
			return err
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\n"); err != nil {
		return err
	}
	for i, item := range value.Items {
		if _, err := fmt.Fprintf(w, "%d. %s\n", i+1, searchConversation(item)); err != nil {
			return err
		}
		body := item.Text
		if body == "" {
			body = item.Snippet
		}
		if err := writeIndentedBody(w, body); err != nil {
			return err
		}
		if item.ChatID != "" {
			if _, err := fmt.Fprintf(w, "  Open: imsgcrawl messages --chat %s\n", item.ChatID); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}
	return nil
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

func senderName(fromMe bool, label string) string {
	if fromMe {
		return "me"
	}
	label = strings.TrimSpace(label)
	if label != "" && label != "them" {
		return label
	}
	return "them"
}

func chatConversation(item archive.ChatSummary) string {
	title := strings.TrimSpace(item.Title)
	people := participantPreview(item.ParticipantHandles, item.ParticipantCount)
	if item.Kind == "group" {
		switch {
		case title != "" && people != "":
			return title + " (" + people + ")"
		case title != "":
			return title
		case people != "":
			return "group with " + people
		default:
			return "group chat"
		}
	}
	if title != "" {
		return title
	}
	if people != "" {
		return people
	}
	if item.ChatID != "" {
		return "chat " + item.ChatID
	}
	return "unknown chat"
}

func searchConversation(item archive.SearchResult) string {
	chat := archive.ChatSummary{
		ChatID:             item.ChatID,
		Title:              item.ChatTitle,
		Kind:               item.ChatKind,
		ParticipantCount:   item.ChatParticipantCount,
		ParticipantHandles: item.ChatParticipantHandles,
	}
	return chatConversation(chat)
}

func participantPreview(handles []string, total int64) string {
	if len(handles) == 0 {
		if total > 0 {
			return fmt.Sprintf("%d people", total)
		}
		return ""
	}
	limit := len(handles)
	if limit > 4 {
		limit = 4
	}
	parts := append([]string{}, handles[:limit]...)
	if remaining := int(total) - limit; remaining > 0 {
		parts = append(parts, fmt.Sprintf("+%d more", remaining))
	}
	return strings.Join(parts, ", ")
}

func writeIndentedBody(w io.Writer, value string) error {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	if strings.TrimSpace(value) == "" {
		_, err := io.WriteString(w, "  (empty)\n")
		return err
	}
	value = strings.TrimRight(value, "\n")
	for _, line := range strings.Split(value, "\n") {
		if _, err := fmt.Fprintf(w, "  %s\n", line); err != nil {
			return err
		}
	}
	return nil
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
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
