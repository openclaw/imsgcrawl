# imsgcrawl 💬 — Your messages, locally searchable

[![CI](https://img.shields.io/github/actions/workflow/status/openclaw/imsgcrawl/ci.yml?branch=main&style=flat-square&label=ci)](https://github.com/openclaw/imsgcrawl/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/openclaw/imsgcrawl?style=flat-square)](https://github.com/openclaw/imsgcrawl/releases/latest)
[![Platforms](https://img.shields.io/badge/platforms-macOS%20%7C%20Linux-555555?style=flat-square)](#install)
[![Go](https://img.shields.io/badge/Go-1.26.5%2B-00ADD8?style=flat-square)](https://go.dev/)
[![License](https://img.shields.io/github/license/openclaw/imsgcrawl?style=flat-square)](#license)

`imsgcrawl` reads Apple Messages through a temporary read-only SQLite snapshot and syncs it into a local archive. It gives people, scripts, and agents bounded commands for checking status, listing chats, reading messages, searching text, and exporting phone contacts.

```text
$ imsgcrawl search "older hello"
Search "older hello": showing 1 of 1.
Use --json when you need local chat IDs for follow-up commands.

date              from        conversation  text
2000-12-31 16:00  Older Name  Older Name    older hello
```

The example uses synthetic fixture data. `imsgcrawl` keeps the source database and archive on the local machine.

## Install

Download a prebuilt archive for macOS or Linux from the [latest release](https://github.com/openclaw/imsgcrawl/releases/latest), extract it, and place `imsgcrawl` on your `PATH`. macOS release binaries are signed and notarized.

To build the latest release from source, install Go 1.26.5 or newer and run:

```sh
go install github.com/openclaw/imsgcrawl/cmd/imsgcrawl@latest
```

## Quick start

The default source is `~/Library/Messages/chat.db`, and the default archive is `~/.imsgcrawl/archive.db`. The shell or terminal running `imsgcrawl` may need Full Disk Access on macOS.

```sh
imsgcrawl status
imsgcrawl sync
imsgcrawl chats --limit 10
imsgcrawl messages --chat CHAT_ID --limit 20
imsgcrawl search "phrase"
```

Take `CHAT_ID` from the `chats` output. List and search commands are bounded by default; use `--all` only when complete local output is intentional.

## Commands

| Command | Purpose |
|---|---|
| `metadata` | Print crawler capabilities and paths |
| `status` | Check source and archive readability without printing messages |
| `sync` | Merge the Messages snapshot into the local archive |
| `chats` | List archived conversations and their local IDs |
| `messages --chat ID` | Read one archived conversation |
| `search QUERY` | Search archived message text |
| `contacts export` | Export display names and normalized phone numbers |

Run `imsgcrawl help COMMAND` for flags. The [command reference](docs/commands.md) includes limits, examples, output shapes, and custom database/archive paths.

## Archive behavior

`sync` merges by default, retains rows absent from an ordinary snapshot, and binds an archive to one normalized Messages database path. `sync --restore` replaces the archive from the current source when changing sources or intentionally rebuilding it.

Explicit Messages deletion feeds become tombstones, and iMessage edits and retractions become append-only revision events. Normal reads and search omit deleted or withdrawn content. See [Archive and retention](docs/archive-and-retention.md) for the data model and restore rules.

## JSON and automation

Add `--json` anywhere in the command line for stable fields and local IDs:

```sh
imsgcrawl --json chats --limit 20 | jq '.items[0]'
```

Text output is for local reading; JSON is for scripts, tests, CrawlBar, and other CrawlKit consumers. The [command reference](docs/commands.md#json-output) shows the response envelope.

## Privacy

Messages data contains private names, email addresses, phone numbers, conversation text, and local paths. Do not publish output from a real Messages database. Public examples, tests, bug reports, and screenshots should use synthetic data.

The archive and smoke-test artifacts contain the same private material. Keep them local unless every disclosed item and destination have been explicitly approved.

## Development

```sh
go mod verify
go test ./...
go vet ./...
mkdir -p ./bin
go build -trimpath -o ./bin/imsgcrawl ./cmd/imsgcrawl
```

The optional devenv shell also provides Go, SQLite, and `jq`. See the [agent smoke transcript](docs/commands.md#agent-smoke-transcript) for an end-to-end local CLI check.

## License

This repository does not currently include a license file.
