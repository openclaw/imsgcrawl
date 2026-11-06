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

## Smoke fixture database cleanup

- Close all five synthetic source connections after their transaction commits.
  Preserve every fixture value, CLI call and assertion.
- The original built-CLI smoke passes while all five observed Python connections
  remain usable. The same smoke now passes with all five connections closed.
- Validate before/after against one temporary CLI binary, transcript privacy tests,
  race-enabled module tests, static checks, independent review and platform CI.
  No live Messages database or release artifact is used.

## Attributed-body round-trip coverage

- Remove the separate three-case `TestDecodeAttributedBody` table. Its short,
  ordinary and 300-byte bodies repeat the round-trip assertion already exercised
  by `FuzzDecodeAttributedBodyRoundTrip` during ordinary `go test` runs.
- Keep every fuzz seed, including leading NULs, Unicode and the 127/128,
  255/256 and 65,535/65,536-byte length boundaries. Preserve the independent
  Foundation-generated fixture, malformed input and overflow checks.
- Compare healthy runs and decoder-fault overlays before/after removal; require
  the retained seed corpus to reject the same short and extended-length faults.
  Validate full module race tests, static checks, built-CLI synthetic smoke,
  independent review and platform CI. No production behavior or release changes.

## Current-message extraction fixtures

- Fold three current-text extraction tests into one table, removing nineteen
  test lines. Preserve each input, schema, isolated SQLite database, current-body
  assertion and malformed-summary fallback check.
- Keep revision metadata, legacy-column availability and malformed-body extraction
  tests separate. They exercise different source shapes and failure paths.
- Eight before/after race suites retain detection of a lost current-body flag,
  exposed known-edit fallback and corrupted decoded text. Healthy suites pass;
  production source is restored byte-for-byte.
- Require full module race tests, static checks, built-CLI synthetic smoke,
  independent review and platform CI. No live Messages data or releases.

## CLI list-limit fixtures

- Remove the populated Messages fixture and sync from six argument-error checks.
  Require each intended usage error against missing temporary source/archive
  paths, preserving the conflicting-flags and zero-limit cases.
- Replace the two non-binding `--all` assertions with one minimal source fixture:
  51 chats and 21 messages. Check default truncation and complete unlimited output
  for chats, messages and search, including counts, limit and completion fields.
- Eight before/after CLI race suites retain both validation-guard checks. Ignoring
  `--all` passes the old entire CLI suite and fails all three repaired command
  cases. Healthy suites pass; production source is restored byte-for-byte.
- Require full module/static checks, built-CLI synthetic smoke, independent review
  and platform CI. Temporary synthetic data only; no production changes or releases.

## Text-table content assertions

- Replace two ellipsis-only wrapping checks with one table that requires the
  complete rendered headers, sender, search context and message text. Use
  reserved-domain synthetic identities long enough to wrap in both layouts.
- Require the exact small padding fixture output so header-only output cannot
  satisfy its old negative assertion. Keep continuation-line spacing unchanged.
- Twelve before/after race suites retain trailing-padding detection and now
  reject header-only output, lost senders, lost search context and disabled
  message-sender wrapping. Healthy suites pass; both production files are restored.
- Require full module/static checks, built-CLI synthetic smoke, independent review
  and platform CI. No production changes, live Messages data or releases.

## Contacts help dispatch

- Remove the unreachable export-level help guard. Its only caller checks the
  same help-before-terminator condition before dispatching the export suffix.
  Keep the parent guard and existing public help output.
- Fold four nonempty-output help checks into the command-help table, preserving
  every input while requiring the appropriate command description. This removes
  thirteen test lines and three production lines without dropping a case.
- Six before/after full-CLI race suites retain parent-help failure detection and
  now reject unrelated nonempty replies for all four cases. A separate poisoned
  child-guard probe passes the original suite, alongside the sole-caller proof.
- Require full module/static checks, built-CLI synthetic smoke, independent review
  and platform CI. Temporary synthetic data only; no live Messages data or releases.
