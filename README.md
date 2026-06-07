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

The transcript contains raw local Messages-derived output. Keep it local unless
the user explicitly asks to share it.

## Commands

```bash
imsgcrawl metadata
imsgcrawl status
imsgcrawl sync
imsgcrawl chats
imsgcrawl chats --all
imsgcrawl messages --chat 123 --limit 50
imsgcrawl search --limit 20 "launch notes"
imsgcrawl contacts export
```

Default output is compact text for humans and agents: list commands say how
many rows are shown, whether more rows exist, and which command to run next.
Use `--all` only when you explicitly want complete local output.

Use `--json` for machine parsing:

```bash
imsgcrawl --json metadata
imsgcrawl --json status
imsgcrawl --json sync
imsgcrawl --json chats --limit 20
imsgcrawl --json messages --chat 123 --limit 50
imsgcrawl --json messages --chat 123 --all
imsgcrawl --json search --limit 20 "launch notes"
imsgcrawl --json search --all "launch notes"
imsgcrawl --json contacts export
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
