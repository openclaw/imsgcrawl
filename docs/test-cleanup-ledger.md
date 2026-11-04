# Test cleanup ledger

## CLI contract tests

- Removed `TestRunEndToEnd`: it called `Run` in process, created a Messages
  database for help/version/metadata, and repeated shallow substring checks.
- Kept no-command help and version checks without a database fixture.
- Moved both JSON flag positions into the full metadata and contact-export
  contract tests. Contact export now checks row count as well as normalized
  values, so duplicate rows cannot disappear into the assertion map.
- Kept source message counts in the existing archive-state status test.
- Retained `scripts/smoke-cli.py` as the built-binary integration check. It uses
  temporary synthetic Messages data and verifies the source bytes stay unchanged.

This is a bounded CLI-test cleanup, not a complete repository audit.
No production behavior or release artifacts changed.

## Sync-lock test waits

- Retained the real file-lock contention, import serialization, extraction-error
  release, and dangling-symlink coverage. These tests use temporary archives.
- Bounded the first sync and its extraction/import channel waits in the two
  serialization tests. An early sync failure now reports the returned error; a
  stalled phase reports its name instead of hanging until the suite timeout.
- Made the paused synthetic extractors observe cancellation, so a failed test
  can release its own lock. The separate 100 ms competing-sync deadlines remain.

Validation: race-enabled archive tests, full module tests and vet, built-CLI
synthetic smoke, and CI. No production code, live Messages data, or release
artifacts changed.

## Snapshot cancellation fixture

- Removed the cancellation check against an already-corrupt SQLite file: any
  corruption error satisfied it, even when the snapshot wrapper ignored context.
- Reuse the valid WAL snapshot fixture and require `context.Canceled`. Keep the
  committed-row, source-byte preservation, corruption, and owned-directory cleanup
  checks. Close any unexpectedly successful cancellation result before failing.
- A temporary production overlay that discards the caller's context passes the
  original snapshot tests and fails the repaired cancellation assertion. Normal
  race tests, static checks, built-CLI synthetic smoke, and CI validate the change.
