---
written_by: ai
---

# imsgcrawl

`imsgcrawl` is a local-first iMessage source crawler. It reads the local
Messages SQLite database through a temporary read-only snapshot, syncs a small
source-native archive, and exposes crawlkit-style metadata, status, read, search,
and contact-export commands.

## Development Shell

Use `devenv` for local development. The shell provides Go, SQLite, `jq`, and a
repo-local `GOBIN` at `.devenv/bin` on `PATH`.

With `direnv` enabled in zsh, the shell activates automatically when you enter
the repo. Activation only prepares the toolchain; build and run commands stay
explicit.

```bash
direnv allow
go install ./cmd/imsgcrawl
imsgcrawl --json status
```

Without `direnv`, enter it manually:

```bash
devenv shell
go install ./cmd/imsgcrawl
imsgcrawl --json status
```

For one-shot checks without entering an interactive shell:

```bash
devenv shell -q -- go install ./cmd/imsgcrawl
./.devenv/bin/imsgcrawl --json status
```

This keeps interactive testing close to the installed product: run the real
`imsgcrawl` command, not wrapper scripts. Re-run `go install ./cmd/imsgcrawl`
after code changes. Use the installed binary directly for clean JSON piping,
because `devenv shell -- <command>` can print shell-manager notices before the
command output.

## Agent Smoke Transcript

Use the smoke transcript when reviewing whether the CLI actually works for an
agent. It runs the real `imsgcrawl` on `PATH`, uses a temporary archive, and
writes exact stdout/stderr for progressive text and JSON commands to `/tmp`.

```bash
scripts/agent-smoke-transcript.sh --query "launch notes"
```

The script prints paths to:

- `review.txt`: bounded previews, byte counts, raw file paths, and the agent
  checklist;
- `manifest.jsonl` and `commands.tsv`: command index with exit codes and raw
  stream paths;
- `raw/`: exact stdout/stderr per command.

Use `--inline-raw` only when you explicitly want a full inline transcript in
addition to the raw files. These artifacts contain raw local Messages-derived
output. Keep them local unless the user explicitly asks to share them.

## Commands

These are the agent-facing commands. Keep this section in sync when command
names, flags, defaults, fields, or default text output change. Examples use
fake Trump cinematic universe fixture data, not real Messages output.

```bash
imsgcrawl metadata
imsgcrawl status
imsgcrawl sync
imsgcrawl chats
imsgcrawl chats --all
imsgcrawl messages --chat 42 --limit 20
imsgcrawl search --limit 20 "candles budget"
imsgcrawl contacts export
```

Default output is compact text for humans and agents: list commands say how
many rows are shown, whether more rows exist, and which command to run next.
Use `--all` only when you explicitly want complete local output.

### Text Examples

`metadata` explains what the crawler is and which commands an agent can run:

```text
iMessage Crawl (imsgcrawl)
Local-first iMessage archive crawler.

Capabilities: metadata, status, sync, chats, messages, search, contact-export

Agent-facing commands:
  status          imsgcrawl --json status             check source/archive readiness and aggregate counts
  sync            imsgcrawl --json sync               refresh the local source-native archive
  chats           imsgcrawl --json chats              list archived chats
  messages        imsgcrawl --json messages           read one chat transcript
  search          imsgcrawl --json search             search archived message text
  contact-export  imsgcrawl --json contacts export    export narrow phone contact rows

Machine output: add --json to print the structured manifest.
```

`status` reports aggregate readiness without printing message contents:

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
  Chat-message links: 12
  Messages: 12
```

`sync` refreshes the archive and reports what was imported:

```text
Sync complete

Messages source:
  Database: /Users/example/Library/Messages/chat.db
  Modified: 2026-06-07T09:14:55Z
  Size: 424242 bytes

Local archive:
  Database: /Users/example/.imsgcrawl/archive.db
  Synced: 2026-06-07T09:15:02Z

Archived rows:
  Handles: 6
  Chats: 4
  Participants: 8
  Chat-message links: 12
  Messages: 12
```

`chats` shows bounded rows plus group/direct context:

```text
Chats: showing 3 of 4, newest first.
More: imsgcrawl chats --limit 4
All: imsgcrawl chats --all
Open: imsgcrawl messages --chat CHAT_ID

chat_id  kind    people  messages  latest            title
42       group   4       6         2026-06-07 09:10  Cabinet Group
17       direct  1       3         2026-06-07 08:55  Failing Elon
9        direct  1       2         2026-06-06 22:03  JD Vance
```

`messages` prints a bounded transcript. Message bodies are not truncated:

```text
Messages for chat 42: showing 2 of 6, newest-first.
More: imsgcrawl messages --chat 42 --limit 6
All: imsgcrawl messages --chat 42 --all
Search: imsgcrawl search QUERY

[108] 2026-06-07 09:10 - me - iMessage
  The candles budget is CORRECT. I will not be explaining this again.

[107] 2026-06-07 09:09 - them: JD Vance - iMessage
  Sir, I have prepared bullet points:
  - The hum is louder
  - The couch remains loyal
```

`search` returns full matched text for each bounded result:

```text
Search "candles budget": showing 1 of 1.
Open: imsgcrawl messages --chat CHAT_ID

[108] chat 42 - 2026-06-07 09:10 - me - iMessage
  The candles budget is CORRECT. I will not be explaining this again.
```

`contacts export` defaults to simple text and `--json` exposes the narrow
contact contract:

```text
Donald	+15550100
JD Vance	+15550101
Failing Elon	+15550102
```

Use `--json` for machine parsing:

```bash
imsgcrawl --json metadata
imsgcrawl --json status
imsgcrawl --json sync
imsgcrawl --json chats --limit 20
imsgcrawl --json messages --chat 42 --limit 20
imsgcrawl --json messages --chat 42 --all
imsgcrawl --json search --limit 20 "candles budget"
imsgcrawl --json search --all "candles budget"
imsgcrawl --json contacts export
```

Message/search JSON keeps stable machine fields plus source-readable context:

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
      "message_id": "108",
      "guid": "fake-message-guid",
      "chat_id": "42",
      "sender_label": "me",
      "date": 801223800000000000,
      "service": "iMessage",
      "from_me": true,
      "text": "The candles budget is CORRECT. I will not be explaining this again.",
      "snippet": "The [candles budget] is CORRECT."
    }
  ]
}
```

`metadata` prints the crawlkit control manifest. `status` reports aggregate
readability and row counts without leaking handles. `sync` creates or refreshes
the local archive at `~/.imsgcrawl/archive.db`. `chats`, `messages`, and
`search` read from that archive.

`contacts export` prints the shared v0 contact-export shape:

```json
{
  "contacts": [
    {
      "display_name": "0118 999 881 999 119 725 3",
      "phone_numbers": ["0118 999 881 999 119 725 3"]
    }
  ]
}
```

The v0 contact contract is intentionally narrow: root key `contacts`, with only
`display_name` and `phone_numbers` on each contact. When Messages has no human
name, the current exporter uses the phone number as `display_name`; downstream
importers should treat that as an unnamed phone-only contact rather than a
canonical human name.

## Privacy

Messages data contains private names, phone numbers, emails, and conversation
contents. Do not publish raw output from a real Messages database. Tests and
public examples must use fake fixture data.
