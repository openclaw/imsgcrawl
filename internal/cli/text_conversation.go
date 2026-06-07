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
	dateWidth, fromWidth, textWidth := messageColumnWidths(width)
	return []textColumn{
		{header: "date", width: dateWidth},
		{header: "from", width: fromWidth, wrap: true},
		{header: "text", width: textWidth, wrap: true},
	}
}

func messageColumnWidths(width int) (dateWidth int, fromWidth int, textWidth int) {
	dateWidth = 16
	gaps := 2 * len(textTableGap)
	remaining := width - dateWidth - gaps
	if remaining < 47 {
		remaining = 47
	}
	fromWidth = clampWidth(remaining*2/5, 18, 30)
	textWidth = remaining - fromWidth
	if textWidth < 30 {
		needed := 30 - textWidth
		fromReduction := minInt(needed, fromWidth-14)
		fromWidth -= fromReduction
		textWidth = remaining - fromWidth
	}
	return dateWidth, fromWidth, textWidth
}

func searchTextColumns(width int) []textColumn {
	dateWidth, fromWidth, conversationWidth, textWidth := searchColumnWidths(width)
	return []textColumn{
		{header: "date", width: dateWidth},
		{header: "from", width: fromWidth, wrap: true},
		{header: "conversation", width: conversationWidth, wrap: true},
		{header: "text", width: textWidth, wrap: true},
	}
}

func searchColumnWidths(width int) (dateWidth int, fromWidth int, conversationWidth int, textWidth int) {
	dateWidth = 16
	gaps := 3 * len(textTableGap)
	remaining := width - dateWidth - gaps
	if remaining < 50 {
		remaining = 50
	}
	fromWidth = clampWidth(remaining/4, 12, 24)
	conversationWidth = clampWidth(remaining/3, 18, 52)
	textWidth = remaining - fromWidth - conversationWidth
	if textWidth < 20 {
		needed := 20 - textWidth
		conversationReduction := minInt(needed, conversationWidth-18)
		conversationWidth -= conversationReduction
		needed -= conversationReduction
		fromReduction := minInt(needed, fromWidth-12)
		fromWidth -= fromReduction
		textWidth = remaining - fromWidth - conversationWidth
	}
	return dateWidth, fromWidth, conversationWidth, textWidth
}

func clampWidth(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
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
