package cli

import (
	"github.com/openclaw/crawlkit/control"
	"github.com/openclaw/imsgcrawl/internal/archive"
)

const (
	defaultChatLimit    = 50
	defaultMessageLimit = 20
	defaultSearchLimit  = 20
)

type listHeader struct {
	SchemaVersion string `json:"schema_version"`
	AppID         string `json:"app_id"`
	Command       string `json:"command"`
	Returned      int    `json:"returned"`
	Total         int64  `json:"total"`
	Limit         int    `json:"limit,omitempty"`
	Complete      bool   `json:"complete"`
}

type chatListOutput struct {
	listHeader
	Items []archive.ChatSummary `json:"items"`
}

type messageListOutput struct {
	listHeader
	ChatID string               `json:"chat_id"`
	Chat   *archive.ChatSummary `json:"chat,omitempty"`
	Order  string               `json:"order"`
	Items  []archive.MessageRow `json:"items"`
}

type searchListOutput struct {
	listHeader
	Query string                 `json:"query"`
	Items []archive.SearchResult `json:"items"`
}

func newListHeader(command string, returned int, total int64, limit int) listHeader {
	return listHeader{
		SchemaVersion: control.SchemaVersion,
		AppID:         "imsgcrawl",
		Command:       command,
		Returned:      returned,
		Total:         total,
		Limit:         limit,
		Complete:      total <= int64(returned),
	}
}
