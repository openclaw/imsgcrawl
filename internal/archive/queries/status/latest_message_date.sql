select coalesce(max(date), 0)
from messages
where deleted_at is null
