# CLI sweep 1051 — env-var batch of fifty (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- A single PUT batch of 50 env vars → 200, and the full set reads back (50/50 present under `limit=100`). No batch-size cliff at this scale.
