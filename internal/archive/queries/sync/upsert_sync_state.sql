insert into sync_state(
  key,
  value
) values(?, ?)
on conflict(key) do update set value = excluded.value
