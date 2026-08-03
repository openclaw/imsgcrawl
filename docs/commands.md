# Command reference

`imsgcrawl` prints bounded text by default and stable JSON with `--json`. Examples in this document use synthetic fixture data.

## Global flags

| Flag | Meaning |
|---|---|
| `--json` | Print machine-readable JSON |
| `--db PATH` | Read a Messages database other than `~/Library/Messages/chat.db` |
| `--archive PATH` | Use an archive other than `~/.imsgcrawl/archive.db` |
| `--version` | Print the binary version |

Use `imsgcrawl help COMMAND` or `imsgcrawl COMMAND --help` for command-specific help.

## Metadata

`metadata` describes the crawler surface without reading message contents:

```sh
imsgcrawl metadata
imsgcrawl --json metadata
```

```text
iMessage Crawl (imsgcrawl)
Local-first iMessage archive crawler.

Capabilities: metadata, status, sync, chats, messages, search, contact-export
```

## Status and sync

`status` checks whether the source and archive are readable and reports aggregate counts. It does not print message contents.

```sh
imsgcrawl status
imsgcrawl sync
imsgcrawl sync --restore
```

```text
Status: ok
Messages source and archive are readable.

Messages source:
  Database: /Users/example/Library/Messages/chat.db
  Handles: 6
  Chats: 4
  Messages: 4

Local archive:
  Database: /Users/example/.imsgcrawl/archive.db
  Last sync: 2026-08-03T01:12:41Z
  Handles: 6
  Chats: 4
  Participants: 6
  Chat-message links: 5
  Messages: 4
```

Normal syncs merge the current snapshot. `--restore` intentionally replaces the archive; see [Archive and retention](archive-and-retention.md).

## Chats

```sh
imsgcrawl chats
imsgcrawl chats --limit 10
imsgcrawl chats --all
```

The default limit is 50. Each row includes the chat ID needed by `messages --chat`.

```text
Chats: showing 4 of 4, newest first.
Open: imsgcrawl messages --chat CHAT_ID

chat  kind    msgs  latest            conversation
4     group   1     2000-12-31 16:00  Cabinet Group (+15550103, opaque-handle, opaque123)
3     direct  1     2000-12-31 16:00  +15550103
2     direct  2     2000-12-31 16:00  Most Recent Name
1     direct  1     2000-12-31 16:00  Older Name
```

## Messages

```sh
imsgcrawl messages --chat 4
imsgcrawl messages --chat 4 --limit 10
imsgcrawl messages --chat 4 --asc
imsgcrawl messages --chat 4 --all
```

The default limit is 20, ordered newest first. Message bodies stay in the `text` column and are not truncated; row limits control output size.

```text
Messages in Cabinet Group (+15550103, opaque-handle, opaque123) (chat 4): showing 1 of 1, newest-first.
Search: imsgcrawl search QUERY

date              from       text
2000-12-31 16:00  +15550103  group fallback row
```

## Search

```sh
imsgcrawl search "older hello"
imsgcrawl search --limit 10 "older hello"
imsgcrawl search --all "older hello"
```

The default limit is 20. Text output identifies the conversation; JSON also carries local chat IDs for follow-up commands.

```text
Search "older hello": showing 1 of 1.
Use --json when you need local chat IDs for follow-up commands.

date              from        conversation  text
2000-12-31 16:00  Older Name  Older Name    older hello
```

## Contacts

`contacts export` returns display names and phone numbers from the Messages source. It does not import Apple Contacts or emit email addresses.

```sh
imsgcrawl contacts export
imsgcrawl --json contacts export
```

```text
+15550103        +15550103
Most Recent Name  0015550100
```

## JSON output

Place `--json` anywhere in the command line:

```sh
imsgcrawl --json status
imsgcrawl --json chats --limit 20
imsgcrawl --json messages --chat 42 --limit 20
imsgcrawl --json search --limit 20 "candles budget"
imsgcrawl --json contacts export
```

List and search responses include `returned`, `total`, `limit`, and `complete`. A search response has this shape:

```json
{
  "schema_version": "crawlkit.control.v1",
  "app_id": "imsgcrawl",
  "command": "search",
  "returned": 1,
  "total": 1,
  "limit": 20,
  "complete": true,
  "query": "candles budget",
  "items": [
    {
      "message_id": "3",
      "guid": "message-three",
      "chat_id": "2",
      "chat_title": "Most Recent Name",
      "chat_kind": "direct",
      "chat_participant_count": 1,
      "chat_participant_handles": ["0015550100"],
      "handle_id": "2",
      "sender_handle": "0015550100",
      "sender_label": "me",
      "date": 250,
      "service": "SMS",
      "from_me": true,
      "text": "latest launch note with candles budget and tariffs. This sentence keeps going so transcript output must stay whole. This sentence keeps going so transcript output must stay whole. This sentence keeps going so transcript output must stay whole. full tail marker",
      "snippet": "latest launch note with [candles] [budget] and tariffs. This sentence keeps going..."
    }
  ]
}
```

## Agent smoke transcript

The smoke script runs the real `imsgcrawl` binary on `PATH`, uses a temporary archive, and captures exact stdout and stderr under `/tmp` by default. Install `jq` before running it.

```sh
go install ./cmd/imsgcrawl
scripts/agent-smoke-transcript.sh --query "candles budget"
```

The script prints paths to `review.txt`, `manifest.jsonl`, `commands.tsv`, and raw stream files. These artifacts contain Messages-derived data. Keep them local unless the user explicitly approves the content and destination.
