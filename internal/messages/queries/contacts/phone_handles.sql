select
  h.id,
  h.service,
  coalesce((
    select nullif(trim(c.display_name), '')
    from chat_handle_join chj
    join chat c on c.rowid = chj.chat_id
    where chj.handle_id = h.rowid
      and (select count(*) from chat_handle_join x where x.chat_id = chj.chat_id) = 1
      and nullif(trim(c.display_name), '') is not null
    order by c.rowid desc
    limit 1
  ), '') as display_name,
  count(m.rowid) as messages,
  coalesce(max(m.date), 0) as last_message
from handle h
left join message m on m.handle_id = h.rowid
where h.id not like '%@%'
group by h.rowid, h.id, h.service
