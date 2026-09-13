# Changelog

## Unreleased

- Update the SQLite libc runtime to v1.75.7 and verify macOS/Linux builds with race detection, synthetic CLI smoke tests, and complete frozen advisory scans; pin CI and release Actions to reviewed commits.

## 0.2.1 - 2026-09-11

**Highlights:** Safer archive access and lower temporary snapshot allocation on supported macOS filesystems.

- Protect Messages sources by rejecting foreign archives, source/sidecar aliases, orphaned or hardlinked sidecars, and hardlinked sync outputs before writable archive access; thanks @vincentkoc (#16).
- Reduce macOS snapshot allocation with private copy-on-write clones on supporting same-volume filesystems while retaining generation hashes, private recovery, and portable byte copies (#19); thanks @vincentkoc for the report (#18).
- Validate and lock the filename SQLite actually opens, including symlink/`..` paths and trailing-whitespace normalization, without changing literal Messages source filenames; thanks @vincentkoc (#16).
- Verify snapshot generations without opening or repairing the Messages source, honor cancellation, and serialize complete syncs so older extractions cannot overwrite newer imports; thanks @vincentkoc (#16).
- Require Go 1.27.0 and macOS 13 for new source builds, preferring the Go 1.27.1 toolchain; previously released binaries retain their original requirements.
- Update CrawlKit to v0.16.1, SQLite to v1.58.0, go-runewidth to v0.0.30, and x/sys to v0.48.0; refresh SQLite runtime dependencies and CI checkout/setup-go, deadcode, vulnerability scanner, and artifact-upload actions.
- Verify both supported Go compiler versions and complete frozen-database vulnerability results in macOS CI.
- Add the MIT license and correct the README license reference; thanks @vincentkoc (#13).

## v0.2.0 - 2026-08-14

### Archive and retention

- Make sync merge by default, bind archives to one Messages source, add explicit `sync --restore` replacement, and retain source-attributed tombstones for chats, messages, and subordinate relationships
- Preserve iMessage edits and unsends as stable append-only message events, reconstruct current non-retracted message bodies, and keep tombstoned rows out of normal reads and search
- Migrate v0.1 archives in place to the revision-aware tombstone schema

### Dependencies

- Add `howett.net/plist` to distinguish iMessage per-part edit and unsend metadata in binary property lists
- Update terminal and system dependencies to `go-runewidth` v0.0.27, `go-isatty` v0.0.24, and `x/sys` v0.47.0
## 0.1.1 - 2026-07-18

### Highlights

- Publish platform archives with clean, stable filenames that match the crawler family convention

### Release engineering

- Place GoReleaser binaries in target-only output directories so the unified packager omits internal build IDs and architecture variant suffixes from asset names

## 0.1.0 - 2026-07-18

### Highlights

- Introduce a local-first iMessage crawler with source-native archive synchronization, bounded reading, and privacy-safe read-only Messages snapshots
- Provide human-readable and JSON interfaces for status, chats, messages, search, and phone-only contact export
- Ship official macOS binaries signed by the OpenClaw Foundation and notarized by Apple

### Archive and search

- Synchronize handles, chats, participants, messages, and full-text search into a source-native SQLite archive
- Decode attributed iMessage bodies when plain-text message content is unavailable
- Keep list and search output bounded, terminal-aware, and explicit about follow-up commands

### Automation and privacy

- Expose CrawlKit control metadata and stable JSON output for agents and local automation
- Add context-safe smoke transcripts and fake-data documentation without publishing private Messages content
- Preserve the narrow contact-export contract with deduplicated phone values and no source-specific fields

### Dependencies

- Update CrawlKit to v0.14.3, modernc SQLite to v1.54.0, go-isatty to v0.0.23, and Go to 1.26.5
