---
written_by: ai
---

# Slice 2: turn imsgcrawl into a proper source-native iMessage crawler

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This plan follows `.agent/PLANS.md` in this repository.

## Purpose / Big Picture

After Slice 2, `imsgcrawl` should be more than a contact producer. It should maintain a local source-native archive of iMessage data that can answer status, chat listing, message reads, and search without repeatedly parsing the live Apple database. The user-visible proof is that a person can run:

    imsgcrawl sync
    imsgcrawl --json status
    imsgcrawl --json chats
    imsgcrawl --json messages --chat <chat-id>
    imsgcrawl --json search <query>

and get useful source-native iMessage results from a local archive while preserving privacy and avoiding canonical people logic.

This matters because contact export only solves the clawdex identity-entrypoint problem. A proper crawler should also make iMessage history usable as source evidence for future private source-review work, just as telecrawl and wacrawl own Telegram and WhatsApp source archives.

## Progress

- [x] (2026-06-06 01:05+02:00) Defined Slice 2 as separate from the contact-export slice.
- [ ] Implement only after Slice 1 has passed tests and real local smoke.
- [ ] Design and create the local archive schema.
- [ ] Add `sync`, `chats`, `messages`, and `search` commands.
- [ ] Add privacy-preserving fixtures and archive tests.
- [ ] Smoke on a copied local Messages database and report aggregate proof.

## Surprises & Discoveries

- Observation: iMessage email handles exist and some are missing from clawdex, but Slice 1 cannot carry them.
  Evidence: earlier local aggregate found 17 distinct email-form iMessage handles, 13 with messages, 9 with five or more messages, and 9 missing from clawdex email index.

## Decision Log

- Decision: Slice 2 should use a source-native archive, not direct clawdex writes.
  Rationale: source crawlers own source facts and clawdex owns people. Writing Messages rows directly into clawdex would mix source evidence with canonical identity state.
  Date/Author: 2026-06-06 / Codex.

- Decision: Use source-native names: handles, chats, chat participants, messages, and attachments.
  Rationale: these are Apple Messages concepts visible in the database. They are understandable and avoid model-invented graph or ontology language.
  Date/Author: 2026-06-06 / Codex.

## Outcomes & Retrospective

Not started. This plan is intentionally deferred until Slice 1 is proven.

## Context and Orientation

Slice 1 reads the live Messages database through a temporary snapshot and returns contact rows. Slice 2 should introduce an archive database under an `imsgcrawl` runtime directory, likely `~/.imsgcrawl/archive.db`, so read commands do not need to touch the live Apple database every time.

The live Messages database has at least these source tables: `handle`, `chat`, `chat_handle_join`, `message`, `chat_message_join`, `attachment`, and `message_attachment_join`. A source-native archive should preserve stable Apple IDs such as `ROWID` and `guid` where useful, service names such as `iMessage`, `SMS`, and `RCS`, timestamps, chat membership, and enough message fields for search and evidence. It should not create canonical people or merged identities.

## Plan of Work

First add an internal archive store, probably under `internal/store`, with a small schema:

- `handles`: source handle row id, handle value, service, uncanonicalized value when present.
- `chats`: source chat row id, guid, chat identifier, service name, display name, room name, archive flags.
- `chat_participants`: chat row id plus handle row id.
- `messages`: source message row id, guid, handle row id, chat row id when known, date, service, from-me flag, text, attachment flag, and searchable body.
- `sync_state`: last sync time, source database path, source database modified time, and schema version.

Then add `sync`, which snapshots the live SQLite triad, reads source tables in deterministic order, and replaces or upserts the archive. Keep the first sync simple and full-replace unless real performance evidence requires incremental cursors.

After sync exists, add read commands:

- `status`: report archive counts, source freshness, latest message date, and privacy warnings.
- `chats`: list chats with source ids, display names when available, participant counts, message counts, and latest message date.
- `messages --chat <id>`: list messages for a selected chat with bounded limit and ascending/descending order.
- `search <query>`: use SQLite FTS over message text and return snippets plus source ids.

Only after these commands are stable should the repo consider a TUI or crawlkit snapshot/backup/mirror support.

## Concrete Steps

From the repository root, after Slice 1 passes:

    go test ./...
    imsgcrawl sync
    imsgcrawl --json status
    imsgcrawl --json chats --limit 20
    imsgcrawl --json search test --limit 20

The real local smoke may print private message snippets. If the user asks for raw output in chat, show it there. Do not commit or publish that output.

## Validation and Acceptance

Acceptance requires fixture-backed tests for sync, chats, messages, and search; a real local smoke against a Messages snapshot; status output that distinguishes live source DB readability from archive freshness; and command output that does not leak private data in public docs.

Search must prove that a query returns a source message row with a stable chat id and source id. `messages --chat` must prove that a selected chat can be read without touching the live database after sync.

## Idempotence and Recovery

`sync` should be safe to rerun. A failed sync should not corrupt the previous archive; write into a transaction or temporary archive and swap only after success. If a schema changes, migrate explicitly or rebuild from the live Messages source.

## Artifacts and Notes

The first archive should be boring SQLite. Do not add embeddings, summaries, clustering, remote publishing, or background scheduling in this slice.

## Interfaces and Dependencies

Use `crawlkit/control` for metadata and status shape, `crawlkit/store` for SQLite hygiene, and app-owned packages for Messages schema parsing. If snapshot or backup export becomes necessary, use crawlkit snapshot/mirror mechanics after the archive schema is stable.

Revision note: Initial future-slice plan created alongside Slice 1 to keep the contact exporter from expanding into a full crawler prematurely.

Revision note: Removed an exact local checkout path from the plan so the public repo does not contain a private machine-specific path.
