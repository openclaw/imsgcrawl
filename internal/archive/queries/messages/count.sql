select count(*)
from chat_messages cm
join messages m on m.source_rowid = cm.message_rowid
where cm.chat_rowid = ?
  and cm.deleted_at is null
  and m.deleted_at is null
