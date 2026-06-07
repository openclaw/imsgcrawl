select
  m.rowid,
  m.guid,
  coalesce(m.handle_id, 0),
  coalesce(m.date, 0),
  coalesce(m.service, ''),
  coalesce(m.is_from_me, 0),
  coalesce(m.text, ''),
  coalesce(m.attributedBody, x''),
  case
    when exists(
      select 1
      from message_attachment_join maj
      where maj.message_id = m.rowid
    )
    then 1
    else 0
  end
from message m
order by m.rowid
