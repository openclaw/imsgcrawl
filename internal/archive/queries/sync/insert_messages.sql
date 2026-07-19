insert into messages(
  source_rowid,
  guid,
  handle_rowid,
  date,
  service,
  is_from_me,
  text,
  has_attachments,
  date_edited,
  date_retracted,
  revision_data,
  deleted_at,
  deletion_reason
) values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
on conflict(source_rowid) do update set
  guid = excluded.guid,
  handle_rowid = excluded.handle_rowid,
  date = excluded.date,
  service = excluded.service,
  is_from_me = excluded.is_from_me,
  text = excluded.text,
  has_attachments = excluded.has_attachments,
  date_edited = excluded.date_edited,
  date_retracted = excluded.date_retracted,
  revision_data = excluded.revision_data,
  deleted_at = coalesce(messages.deleted_at, excluded.deleted_at),
  deletion_reason = coalesce(messages.deletion_reason, excluded.deletion_reason)
