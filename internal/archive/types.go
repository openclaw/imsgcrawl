package archive

type SyncResult struct {
	ArchivePath      string `json:"archive_path"`
	SourcePath       string `json:"source_path"`
	SourceBytes      int64  `json:"source_bytes"`
	SourceModifiedAt string `json:"source_modified_at,omitempty"`
	SyncedAt         string `json:"synced_at"`
	Handles          int    `json:"handles"`
	Chats            int    `json:"chats"`
	Participants     int    `json:"participants"`
	ChatMessages     int    `json:"chat_messages"`
	Messages         int    `json:"messages"`
}

type Status struct {
	ArchivePath       string `json:"archive_path"`
	ArchiveBytes      int64  `json:"archive_bytes,omitempty"`
	LastSyncAt        string `json:"last_sync_at,omitempty"`
	SourcePath        string `json:"source_path,omitempty"`
	SourceBytes       int64  `json:"source_bytes,omitempty"`
	SourceModifiedAt  string `json:"source_modified_at,omitempty"`
	Handles           int64  `json:"handles"`
	Chats             int64  `json:"chats"`
	Participants      int64  `json:"participants"`
	ChatMessages      int64  `json:"chat_messages"`
	Messages          int64  `json:"messages"`
	LatestMessageDate int64  `json:"latest_message_date,omitempty"`
}

type ChatSummary struct {
	ChatID             string   `json:"chat_id"`
	GUID               string   `json:"guid"`
	Title              string   `json:"title"`
	Kind               string   `json:"kind"`
	ChatIdentifier     string   `json:"chat_identifier,omitempty"`
	RoomName           string   `json:"room_name,omitempty"`
	Service            string   `json:"service,omitempty"`
	ParticipantCount   int64    `json:"participant_count"`
	ParticipantHandles []string `json:"participant_handles,omitempty"`
	MessageCount       int64    `json:"message_count"`
	LatestMessageDate  int64    `json:"latest_message_date,omitempty"`
}

type MessageRow struct {
	MessageID      string `json:"message_id"`
	GUID           string `json:"guid"`
	ChatID         string `json:"chat_id"`
	HandleID       string `json:"handle_id,omitempty"`
	SenderHandle   string `json:"sender_handle,omitempty"`
	SenderLabel    string `json:"sender_label,omitempty"`
	Date           int64  `json:"date"`
	Service        string `json:"service,omitempty"`
	FromMe         bool   `json:"from_me"`
	Text           string `json:"text,omitempty"`
	HasAttachments bool   `json:"has_attachments,omitempty"`
}

type SearchResult struct {
	MessageID              string   `json:"message_id"`
	GUID                   string   `json:"guid"`
	ChatID                 string   `json:"chat_id,omitempty"`
	ChatTitle              string   `json:"chat_title,omitempty"`
	ChatKind               string   `json:"chat_kind,omitempty"`
	ChatParticipantCount   int64    `json:"chat_participant_count,omitempty"`
	ChatParticipantHandles []string `json:"chat_participant_handles,omitempty"`
	HandleID               string   `json:"handle_id,omitempty"`
	SenderHandle           string   `json:"sender_handle,omitempty"`
	SenderLabel            string   `json:"sender_label,omitempty"`
	Date                   int64    `json:"date"`
	Service                string   `json:"service,omitempty"`
	FromMe                 bool     `json:"from_me"`
	HasAttachments         bool     `json:"has_attachments,omitempty"`
	Text                   string   `json:"text,omitempty"`
	Snippet                string   `json:"snippet,omitempty"`
}
