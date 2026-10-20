# CLI sweep 624 — log window timezones: offset forms and relative windows resolve correctly

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `c8717afdf`, 2026-10-19. Read-only against the baseline service; no fixture, no cleanup. Completes sweep 623's Unrun items.

- **Offset windows hit the same instant:** the known line at `23:38:36Z` was returned by all three spellings of its window — `23:38:30Z–23:38:40Z`, `2026-10-20T01:38:30+02:00 – 01:38:40+02:00` (next calendar day at +02:00), and `18:38:30-05:00 – 18:38:40-05:00`. Server-side window math converts offsets instead of comparing wall-clock strings.
- **Relative window:** `--start 2h --end 1h` returned only lines from that past hour, newest-first under the default backward direction.

## Unrun

- Sub-second RFC3339 windows and boundary inclusivity at the exact line timestamp.
