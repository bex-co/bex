# CLI sweep 997 — cache purge surface absence (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `POST /services/{id}/purge-cache` → 404: no such route. Cache purge is a documented non-goal in the compatibility ledger, so the absence is the designed answer, not a gap.
- The supported cache control is the deploy parameter: `POST …/deploys {clearCache:"clear"}` → 201.
