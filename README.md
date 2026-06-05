---
written_by: ai
---

# imsgcrawl

`imsgcrawl` is a local-first iMessage source crawler.

The first slice exposes iMessage contact handles to clawdex through the shared
crawlkit control contract. It reads the local Messages SQLite database
read-only, exports phone-based contacts, and falls back to the phone number as
the display name when Messages does not have a human-readable name.

## Commands

```bash
imsgcrawl --json metadata
imsgcrawl --json status
imsgcrawl --json contacts export
```

`metadata` prints the crawlkit control manifest. `status` reports aggregate
readability and row counts without leaking handles. `contacts export` prints:

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

## Privacy

Messages data contains private names, phone numbers, emails, and conversation
contents. Do not publish raw output from a real Messages database. Tests and
public examples must use fake fixture data.
