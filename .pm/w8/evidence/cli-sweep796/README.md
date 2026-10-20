# CLI sweep 796 — manual scale bounds on a free service (live, 2026-10-20)

Disposable free web service created, probed, deleted (204, then GET 404). No new bug filed.

- `POST /services/{id}/scale {"numInstances":2}` on free → named 400 `the free plan is limited to 1 instance(s) and cannot scale to 2` — fail-closed, no write.
- `numInstances: 0` and `-1` → named 400 `numInstances must be 1-100` (bounds validated by name).
- `numInstances: 1` → 202; GET confirms `numInstances: 1` (unchanged).
