---
written_by: ai
---

# imsgcrawl

`imsgcrawl` is a local-first iMessage crawler. It reads Apple Messages through
a temporary read-only SQLite snapshot, syncs a source-native archive, and gives
humans or agents a small command surface for status, chat listing, transcript
reading, search, and contact export.

The default output is for humans first and agents second: bounded, readable,
and explicit about follow-up commands. Use `--json` for machines, scripts,
tests, CrawlBar, or other workflows that need stable fields and local IDs.

## Quick Start

```bash
imsgcrawl status
imsgcrawl sync
imsgcrawl chats
imsgcrawl messages --chat 42
imsgcrawl search "candles budget"
imsgcrawl contacts export
```

List commands are bounded by default. They show how many rows were returned and
how to ask for more. Use `--all` only when you really want complete local
output.

## What You See

Examples below use fake Trump cinematic universe fixture data, not real
Messages output.

### Metadata

```text
iMessage Crawl (imsgcrawl)
Local-first iMessage archive crawler.

Capabilities: metadata, status, sync, chats, messages, search, contact-export

Agent-facing commands:
  status          imsgcrawl --json status
  sync            imsgcrawl --json sync
  chats           imsgcrawl --json chats
  messages        imsgcrawl --json messages
  search          imsgcrawl --json search
  contact-export  imsgcrawl --json contacts export

Machine output: add --json to print the structured manifest.
```

### Status And Sync

`status` is a safe readiness check. It reports counts, not message contents.

```text
Status: ok
Messages source and archive are readable.

Messages source:
  Database: /Users/example/Library/Messages/chat.db
  Handles: 6
  Chats: 4
  Messages: 12

Local archive:
  Database: /Users/example/.imsgcrawl/archive.db
  Last sync: 2026-06-07T09:15:02Z
  Handles: 6
  Chats: 4
  Participants: 8
  Messages: 12
```

`sync` refreshes the local archive and prints what it imported.

### Chats

`chats` shows the chat ID needed for `messages --chat`, plus enough context for
a human or agent to choose the right conversation.

```text
Chats: showing 3 of 4, newest first.
More: imsgcrawl chats --limit 4
All: imsgcrawl chats --all
Open: imsgcrawl messages --chat CHAT_ID

chat  kind    msgs  latest            conversation
42    group   6     2026-06-07 09:10  Cabinet Group (Elon, JD, Xi, +1 more)
17    direct  3     2026-06-07 08:55  Failing Elon
9     direct  2     2026-06-06 22:03  JD Vance
```

### Messages

`messages` is a wrapped table for scanning. Message bodies stay in the `text`
column and are not truncated; row limits control size.

```text
Messages in Cabinet Group (chat 42): showing 3 of 6, newest-first.
More: imsgcrawl messages --chat 42 --limit 6
All: imsgcrawl messages --chat 42 --all
Search: imsgcrawl search QUERY

date              from        text
2026-06-07 09:10  me          The candles budget is CORRECT.
2026-06-07 09:09  JD Vance    Sir, I have prepared bullet points:
                              - The hum is louder
                              - The couch remains loyal
2026-06-07 09:08  Failing Elon  (attachment)
```

### Search

`search` shows which conversation a hit came from and keeps the full matched
text in the table. Use JSON when an agent or script needs local chat IDs.

```text
Search "candles budget": showing 1 of 1.
Use --json when you need local chat IDs for follow-up commands.

date              from  conversation                  text
2026-06-07 09:10  me    Cabinet Group (Elon, JD,      The candles budget is CORRECT.
                         Xi, +1 more)
```

### Contacts

`contacts export` is intentionally narrow: display name plus phone numbers.

```text
Donald        +15550100
JD Vance      +15550101
Failing Elon  +15550102
```

## JSON Mode

Text is the agent/human reading surface. JSON is the machine/workflow surface.
It includes local IDs and stable field names, so code can pipe it through `jq`
or feed it to crawlkit/CrawlBar-style consumers.

```bash
imsgcrawl --json status
imsgcrawl --json chats --limit 20
imsgcrawl --json messages --chat 42 --limit 20
imsgcrawl --json search --limit 20 "candles budget"
imsgcrawl --json contacts export
```

Search JSON keeps the envelope small and parseable:

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
      "chat_id": "42",
      "chat_title": "Cabinet Group",
      "sender_label": "me",
      "text": "The candles budget is CORRECT."
    }
  ]
}
```

## Privacy

Messages data contains private names, phone numbers, emails, and conversation
contents. Do not publish raw output from a real Messages database. Tests and
public examples must use fake fixture data.

## Development

The repo is designed to be tested through the real `imsgcrawl` binary.

With Nix/devenv:

```bash
direnv allow
go install ./cmd/imsgcrawl
imsgcrawl status
```

Without Nix, install Go `1.26.2` or newer, then use the normal Go workflow:

```bash
go test ./...
go install ./cmd/imsgcrawl
imsgcrawl status
```

Install `jq` if you want to run the smoke transcript or inspect JSON examples.

## Agent Smoke Transcript

Use the smoke transcript when reviewing whether the CLI actually works for an
agent. It runs the real `imsgcrawl` on `PATH`, uses a temporary archive, and
writes exact stdout/stderr for progressive text and JSON commands to `/tmp`.

```bash
scripts/agent-smoke-transcript.sh --query "candles budget"
```

The script prints paths to `review.txt`, `manifest.jsonl`, `commands.tsv`, and
the raw stdout/stderr directory. These artifacts contain local Messages-derived
output. Keep them local unless the user explicitly asks to share them.
