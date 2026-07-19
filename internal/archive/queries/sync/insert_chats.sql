insert into chats(
  source_rowid,
  guid,
  chat_identifier,
  service_name,
  display_name,
  room_name,
  is_archived,
  deleted_at,
  deletion_reason
) values(?, ?, ?, ?, ?, ?, ?, ?, ?)
on conflict(source_rowid) do update set
  guid = excluded.guid,
  chat_identifier = excluded.chat_identifier,
  service_name = excluded.service_name,
  display_name = excluded.display_name,
  room_name = excluded.room_name,
  is_archived = excluded.is_archived,
  deleted_at = coalesce(chats.deleted_at, excluded.deleted_at),
  deletion_reason = coalesce(chats.deletion_reason, excluded.deletion_reason)
