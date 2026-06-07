select
  chat_id,
  message_id
from chat_message_join
order by chat_id, message_id
