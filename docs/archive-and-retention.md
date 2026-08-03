# Archive and retention

`imsgcrawl` copies the live Messages database into a temporary read-only SQLite snapshot before reading it. `sync` then stores source-native handles, chats, participants, messages, revisions, and search data in the local archive.

## Merge and restore

A normal sync merges the current snapshot into the archive. Rows missing from one snapshot remain in the archive because absence is not a deletion signal.

Each archive is bound to one normalized Messages database path. Pointing it at a different source fails instead of combining unrelated message stores. Use `imsgcrawl sync --restore` only when intentionally replacing the archive with the current source snapshot or changing sources.

## Deletions

The archive retains explicit deletion tombstones from the Messages sync-delete feeds and marks retracted iMessages as unsent. Chats, messages, handles, and their participant or message relationships carry `deleted_at` and `deletion_reason` where applicable. Normal reads and search omit tombstoned rows.

An ordinary incomplete snapshot does not create deletion tombstones. Only explicit source signals do.

## Edits and retractions

Message revisions are append-only events with deterministic identities. The current message row also retains Apple's edit and retraction timestamps plus raw `message_summary_info`.

Per-part `ec` edit history and `rp` retractions distinguish partial changes from fully unsent messages. The archive reconstructs the visible body from the latest edited parts while omitting retracted parts, so withdrawn text is not indexed. Unrelated summary metadata does not create revisions.

## Paths

The defaults are:

| Data | Path |
|---|---|
| Messages source | `~/Library/Messages/chat.db` |
| Local archive | `~/.imsgcrawl/archive.db` |
| Cache | `~/.imsgcrawl/cache` |
| Logs | `~/.imsgcrawl/logs` |

Use `--db PATH` and `--archive PATH` to override the source and archive for one command.

## Privacy boundary

The archive contains message and contact data derived from the source. It is local application data, not a sanitized export. Back it up, inspect it, and disclose it with the same care as the original Messages database.
