insert into messages(
  source_rowid,
  guid,
  handle_rowid,
  date,
  service,
  is_from_me,
  text,
  has_attachments
) values(?, ?, ?, ?, ?, ?, ?, ?)
