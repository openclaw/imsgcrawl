insert into handles(
  source_rowid,
  handle,
  service,
  uncanonicalized_id
) values(?, ?, ?, ?)
on conflict(source_rowid) do update set
  handle = excluded.handle,
  service = excluded.service,
  uncanonicalized_id = excluded.uncanonicalized_id
