select
  m.source_rowid,
  m.guid,
  coalesce(cm.chat_rowid, 0),
  coalesce(nullif(trim(c.display_name), ''), nullif(trim(c.room_name), ''), nullif(trim(c.chat_identifier), ''), c.guid, ''),
  case
    when coalesce(pc.participants, 0) > 1 or nullif(trim(c.room_name), '') is not null then 'group'
    when cm.chat_rowid is null then ''
    else 'direct'
  end,
  coalesce(pc.participants, 0),
  m.handle_rowid,
  coalesce(h.handle, ''),
  m.date,
  coalesce(m.service, ''),
  m.is_from_me,
  m.has_attachments,
  coalesce(m.text, ''),
  coalesce(c.display_name, ''),
  coalesce(pc.participants, 0),
  snippet(messages_fts, 1, '[', ']', '...', 12)
from messages_fts
join messages m on m.source_rowid = messages_fts.source_rowid
left join chat_messages cm on cm.message_rowid = m.source_rowid
left join handles h on h.source_rowid = m.handle_rowid
left join chats c on c.source_rowid = cm.chat_rowid
left join (
  select chat_rowid, count(distinct handle_rowid) as participants
  from chat_participants
  group by chat_rowid
) pc on pc.chat_rowid = cm.chat_rowid
where messages_fts match ?
order by rank, cm.chat_rowid
{{LIMIT}}
