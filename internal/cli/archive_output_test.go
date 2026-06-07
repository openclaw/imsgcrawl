package cli

import "encoding/json"

type chatJSONItem struct {
	ChatID            string `json:"chat_id"`
	Title             string `json:"title"`
	MessageCount      int64  `json:"message_count"`
	LatestMessageDate int64  `json:"latest_message_date"`
}

type chatListJSON struct {
	SchemaVersion string         `json:"schema_version"`
	AppID         string         `json:"app_id"`
	Command       string         `json:"command"`
	Returned      int            `json:"returned"`
	Total         int64          `json:"total"`
	Limit         int            `json:"limit"`
	Complete      bool           `json:"complete"`
	Items         []chatJSONItem `json:"items"`
}

type messageJSONItem struct {
	MessageID      string `json:"message_id"`
	GUID           string `json:"guid"`
	ChatID         string `json:"chat_id"`
	Service        string `json:"service"`
	Text           string `json:"text"`
	FromMe         bool   `json:"from_me"`
	HasAttachments bool   `json:"has_attachments"`
}

type messageListJSON struct {
	SchemaVersion string            `json:"schema_version"`
	AppID         string            `json:"app_id"`
	Command       string            `json:"command"`
	Returned      int               `json:"returned"`
	Total         int64             `json:"total"`
	Limit         int               `json:"limit"`
	Complete      bool              `json:"complete"`
	ChatID        string            `json:"chat_id"`
	Order         string            `json:"order"`
	Items         []messageJSONItem `json:"items"`
}

type searchListJSON struct {
	SchemaVersion string                       `json:"schema_version"`
	AppID         string                       `json:"app_id"`
	Command       string                       `json:"command"`
	Returned      int                          `json:"returned"`
	Total         int64                        `json:"total"`
	Limit         int                          `json:"limit"`
	Complete      bool                         `json:"complete"`
	Query         string                       `json:"query"`
	Items         []map[string]json.RawMessage `json:"items"`
}
