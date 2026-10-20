# CLI sweep 1131 — env values store verbatim (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- Single-key PUTs of an empty string, a value containing a newline, a value containing `=` signs, and a non-ASCII value (`héllo→✓`) all → 200, and the list read-back returns each value byte-for-byte. No trimming, escaping, or charset restriction is applied to env values.
