select
  m.source_rowid,
  m.guid,
  cm.chat_rowid,
  m.handle_rowid,
  coalesce(h.handle, ''),
  m.date,
  coalesce(m.service, ''),
  m.is_from_me,
  coalesce(m.text, ''),
  m.has_attachments,
  coalesce(c.display_name, ''),
  coalesce(pc.participants, 0)
from chat_messages cm
join messages m on m.source_rowid = cm.message_rowid
left join handles h on h.source_rowid = m.handle_rowid and h.deleted_at is null
left join chats c on c.source_rowid = cm.chat_rowid and c.deleted_at is null
left join (
  select chat_rowid, count(distinct handle_rowid) as participants
  from chat_participants
  where deleted_at is null
  group by chat_rowid
) pc on pc.chat_rowid = cm.chat_rowid
where cm.chat_rowid = ?
  and cm.deleted_at is null
  and m.deleted_at is null
order by m.date {{ORDER}}, m.source_rowid {{TIE}}
{{LIMIT}}
