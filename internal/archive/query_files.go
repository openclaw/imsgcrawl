package archive

import (
	_ "embed"
	"strings"
)

//go:embed queries/chats/summary.sql
var chatSummarySQL string

//go:embed queries/chats/participant_handles.sql
var participantHandlesSQL string

//go:embed queries/messages/list.sql
var messagesListSQL string

//go:embed queries/messages/count.sql
var countMessagesSQL string

//go:embed queries/search/list.sql
var searchListSQL string

//go:embed queries/search/count.sql
var countSearchSQL string

//go:embed queries/status/latest_message_date.sql
var latestMessageDateSQL string

//go:embed queries/status/sync_state.sql
var syncStateSQL string

//go:embed queries/sync/insert_handles.sql
var insertHandlesSQL string

//go:embed queries/sync/insert_chats.sql
var insertChatsSQL string

//go:embed queries/sync/insert_chat_participants.sql
var insertChatParticipantsSQL string

//go:embed queries/sync/insert_chat_messages.sql
var insertChatMessagesSQL string

//go:embed queries/sync/insert_messages.sql
var insertMessagesSQL string

//go:embed queries/sync/insert_messages_fts.sql
var insertMessagesFTSSQL string

//go:embed queries/sync/upsert_sync_state.sql
var upsertSyncStateSQL string

func chatSummaryQuery(where string) string {
	return strings.Replace(chatSummarySQL, "{{WHERE}}", where, 1)
}

func messagesQuery(order, tie, limitClause string) string {
	query := strings.ReplaceAll(messagesListSQL, "{{ORDER}}", order)
	query = strings.ReplaceAll(query, "{{TIE}}", tie)
	return strings.Replace(query, "{{LIMIT}}", limitClause, 1)
}

func searchQuery(limitClause string) string {
	return strings.Replace(searchListSQL, "{{LIMIT}}", limitClause, 1)
}
