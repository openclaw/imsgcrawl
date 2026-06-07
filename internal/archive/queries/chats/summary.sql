select
  c.source_rowid,
  c.guid,
  coalesce(nullif(trim(c.display_name), ''), nullif(trim(c.room_name), ''), nullif(trim(c.chat_identifier), ''), c.guid) as title,
  case
    when count(distinct cp.handle_rowid) > 1 or nullif(trim(c.room_name), '') is not null then 'group'
    else 'direct'
  end as kind,
  coalesce(c.chat_identifier, ''),
  coalesce(c.room_name, ''),
  coalesce(c.service_name, ''),
  count(distinct cp.handle_rowid) as participants,
  count(distinct cm.message_rowid) as messages,
  coalesce(max(m.date), 0) as latest_message
from chats c
left join chat_participants cp on cp.chat_rowid = c.source_rowid
left join chat_messages cm on cm.chat_rowid = c.source_rowid
left join messages m on m.source_rowid = cm.message_rowid
{{WHERE}}
group by c.source_rowid, c.guid, c.display_name, c.room_name, c.chat_identifier, c.service_name
order by latest_message desc, c.source_rowid desc
