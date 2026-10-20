# CLI sweep 1219 — cursor walk is disjoint; order/date params validate (live, 2026-10-20)

Read-only sweep over pre-existing services (baseline and older fixtures from other work; nothing created or mutated). No new bug filed.

- Walking the services list with `limit=3` then the returned cursor yields a disjoint second page — cursor pagination neither repeats nor overlaps rows.
- `order=bogus` and a malformed `updatedAfter` → 400: auxiliary list parameters are validated with the same strictness as `limit` (sweep 1117).
