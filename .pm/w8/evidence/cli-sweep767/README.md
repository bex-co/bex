# CLI sweep 767 — env-var newline/unicode round-trip (live, 2026-10-19)

Disposable free web service `qa767-web` (`srv-dbbcost4c21c738t03p0`). Deleted and verified absent (REST 404).

- Value with an embedded newline → stored and read back byte-exact (length 11, real `\n` present — the doubled backslashes in the first console rendering were a display artifact, discarded after the length check).
- Unicode value (`héllo→`, 6 chars) → round-trips unchanged.

No bug filed.
