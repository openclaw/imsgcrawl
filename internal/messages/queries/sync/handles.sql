select
  rowid,
  id,
  service,
  coalesce(uncanonicalized_id, '')
from handle
order by rowid
