select name
from sqlite_master
where type = 'table'
  and name = ?
