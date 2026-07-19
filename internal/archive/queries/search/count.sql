select count(*)
from messages_fts
join messages m on m.source_rowid = messages_fts.source_rowid
left join chat_messages cm on cm.message_rowid = m.source_rowid and cm.deleted_at is null
left join chats c on c.source_rowid = cm.chat_rowid
where messages_fts match ?
  and m.deleted_at is null
  and (
    not exists(select 1 from chat_messages any_cm where any_cm.message_rowid = m.source_rowid)
    or (cm.message_rowid is not null and (c.source_rowid is null or c.deleted_at is null))
  )
