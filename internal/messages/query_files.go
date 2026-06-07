package messages

import _ "embed"

//go:embed queries/common/table_exists.sql
var tableExistsSQL string

//go:embed queries/sync/handles.sql
var extractHandlesSQL string

//go:embed queries/sync/chats.sql
var extractChatsSQL string

//go:embed queries/sync/participants.sql
var extractParticipantsSQL string

//go:embed queries/sync/chat_messages.sql
var extractChatMessagesSQL string

//go:embed queries/sync/messages.sql
var extractMessagesSQL string

//go:embed queries/status/handle_ids.sql
var handleIDsSQL string

//go:embed queries/contacts/phone_handles.sql
var phoneHandleRowsSQL string
