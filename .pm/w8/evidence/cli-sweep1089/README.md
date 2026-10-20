# CLI sweep 1089 — concurrent same-name creates: exactly one wins (live, 2026-10-20)

One disposable free web service created by the race winner and deleted (204; name list empty). No new bug filed.

- Two simultaneous creates with the same name → one 201, one 409 `CONFLICT`. Name uniqueness holds under concurrency, not just sequentially (sweep 988) — no duplicate pair slipped through.
