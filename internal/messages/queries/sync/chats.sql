select
  rowid,
  guid,
  coalesce(chat_identifier, ''),
  coalesce(service_name, ''),
  coalesce(display_name, ''),
  coalesce(room_name, ''),
  coalesce(is_archived, 0)
from chat
order by rowid
