# CLI sweep 1039 — blueprint reads; baseline rename probe reverted (live, 2026-10-20)

Read probes on the baseline blueprint, plus one operator mistake, reverted in-sweep. No new bug filed.

- `GET /blueprints/{id}/syncs` → 200; the baseline blueprint reads `status: created`, `name: bex`.
- Mistake recorded honestly: a "PATCH bogus" probe was aimed at the baseline blueprint instead of a disposable fixture, and blueprint rename is a real operation — the baseline's name was briefly `qa1039-rename`. It was immediately PATCHed back to `bex` and re-read to confirm (`name: bex`, `status: created`, manifest and sync state untouched — the rename is display-name only). The baseline is restored; future probes of this class use disposable fixtures or refusals only.
