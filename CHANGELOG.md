# Changelog

## Unreleased

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
