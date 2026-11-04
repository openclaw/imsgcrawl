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
