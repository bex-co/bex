# CLI sweep 1298 — free-plan scaling limits (live, 2026-10-21)

Production `https://api.bex.co/v1/`, pin `f64d4c85f2cd`, bex-canary. A free echo web service (`srv-dbcgd6kiimkc73f6l1lg`, `numInstances: 1`):

- `POST /v1/services/{id}/scale {"numInstances":2}` → 400 `the free plan is limited to 1 instance(s) and cannot scale to 2`.
- `… {"numInstances":0}` → 400 `numInstances must be 1-100`.
- `… {"numInstances":1}` → 202. Bex returns the service object as a body, where Render's schema has `202` with no content. That superset is harmless.
- `PUT /v1/services/{id}/autoscaling` → 400 `autoscaling requires a paid instance type`.
- These match Render's free-instance limits. Deleted (204 → 404). No new bug.
