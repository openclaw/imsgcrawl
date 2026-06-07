package cli

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/openclaw/imsgcrawl/internal/archive"
)

const objectReplacementCharacter = "\uFFFC"

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

func chatTextColumns(width int) []textColumn {
	chatWidth := 6
	kindWidth := 6
	msgsWidth := 5
	latestWidth := 16
	conversationWidth := textColumnWidth(width, chatWidth, kindWidth, msgsWidth, latestWidth)
	return []textColumn{
		{header: "chat", width: chatWidth},
		{header: "kind", width: kindWidth},
		{header: "msgs", width: msgsWidth},
		{header: "latest", width: latestWidth},
		{header: "conversation", width: conversationWidth, wrap: true},
	}
}

func messageTextColumns(width int) []textColumn {
	dateWidth := 16
	fromWidth := 18
	serviceWidth := 16
	if width >= 100 {
		fromWidth = 22
	}
	textWidth := textColumnWidth(width, dateWidth, fromWidth, serviceWidth)
	return []textColumn{
		{header: "date", width: dateWidth},
		{header: "from", width: fromWidth},
		{header: "service", width: serviceWidth},
		{header: "text", width: textWidth, wrap: true},
	}
}

func searchTextColumns(width int) []textColumn {
	dateWidth := 16
	fromWidth := 16
	conversationWidth := 30
	if width < 100 {
		fromWidth = 14
		conversationWidth = 28
	}
	if width < 80 {
		fromWidth = 12
		conversationWidth = 22
	}
	textWidth := textColumnWidth(width, dateWidth, fromWidth, conversationWidth)
	return []textColumn{
		{header: "date", width: dateWidth},
		{header: "from", width: fromWidth},
		{header: "conversation", width: conversationWidth},
		{header: "text", width: textWidth, wrap: true},
	}
}

func messageService(service string, hasAttachments bool) string {
	service = strings.TrimSpace(service)
	if hasAttachments {
		if service == "" {
			return "attachment"
		}
		return service + " +attach"
	}
	return service
}

func searchText(item archive.SearchResult) string {
	if item.Text != "" {
		return displayMessageText(item.Text, item.HasAttachments)
	}
	if item.Snippet != "" {
		return displayMessageText(item.Snippet, item.HasAttachments)
	}
	if item.HasAttachments {
		return "(attachment)"
	}
	return ""
}

func displayMessageText(text string, hasAttachments bool) string {
	if hasAttachments && strings.TrimSpace(strings.ReplaceAll(text, objectReplacementCharacter, "")) == "" {
		return "(attachment)"
	}
	return strings.ReplaceAll(text, objectReplacementCharacter, "[attachment]")
}

func chatConversation(item archive.ChatSummary) string {
	title := strings.TrimSpace(item.Title)
	if isMachineChatTitle(title) {
		title = ""
	}
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

func isMachineChatTitle(title string) bool {
	title = strings.ToLower(strings.TrimSpace(title))
	if len(title) >= 8 && strings.HasPrefix(title, "chat") && allRunes(title[4:], unicode.IsDigit) {
		return true
	}
	if len(title) >= 16 && allRunes(title, isHexRune) {
		return true
	}
	return false
}

func allRunes(value string, match func(rune) bool) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !match(r) {
			return false
		}
	}
	return true
}

func isHexRune(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
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
