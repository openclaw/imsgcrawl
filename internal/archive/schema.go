package archive

const schemaVersion = 2

const schema = `
create table if not exists handles (
  source_rowid integer primary key,
  handle text not null,
  service text not null,
  uncanonicalized_id text,
  deleted_at text,
  deletion_reason text
);

create table if not exists chats (
  source_rowid integer primary key,
  guid text not null,
  chat_identifier text,
  service_name text,
  display_name text,
  room_name text,
  is_archived integer not null default 0,
  deleted_at text,
  deletion_reason text
);

create table if not exists chat_participants (
  chat_rowid integer not null,
  handle_rowid integer not null,
  deleted_at text,
  deletion_reason text,
  primary key (chat_rowid, handle_rowid)
);

create table if not exists chat_messages (
  chat_rowid integer not null,
  message_rowid integer not null,
  deleted_at text,
  deletion_reason text,
  primary key (chat_rowid, message_rowid)
);

create table if not exists messages (
  source_rowid integer primary key,
  guid text not null,
  handle_rowid integer not null default 0,
  date integer not null default 0,
  service text,
  is_from_me integer not null default 0,
  text text,
  has_attachments integer not null default 0,
  date_edited integer not null default 0,
  date_retracted integer not null default 0,
  revision_data blob,
  deleted_at text,
  deletion_reason text
);

create table if not exists message_events (
  event_key text primary key,
  message_guid text not null,
  source_rowid integer not null,
  event_type text not null,
  revision_at integer not null default 0,
  payload_json text not null,
  revision_data blob,
  observed_at text not null,
  deleted_at text,
  deletion_reason text
);

create virtual table if not exists messages_fts using fts5(source_rowid unindexed, text);

create table if not exists sync_state (
  key text primary key,
  value text not null
);

create index if not exists idx_chat_messages_chat on chat_messages(chat_rowid, message_rowid);
create index if not exists idx_chat_messages_message on chat_messages(message_rowid, chat_rowid);
create index if not exists idx_messages_date on messages(date, source_rowid);
create index if not exists idx_messages_guid on messages(guid);
create index if not exists idx_message_events_message on message_events(message_guid, revision_at, event_key);
`
