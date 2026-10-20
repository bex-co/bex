# CLI sweep 889 — delete-immediately and repeat-delete convergence (live, 2026-10-20)

Disposable free web service created and deleted within seconds (204; GET 404; name list `[]`). No new bug filed.

- DELETE issued immediately after create (deploy still opening) → 204 and the resource is absent on the very next read — no window where a just-deleted service still answers (the m81 consistent-absence contract at the tightest timing this loop has probed).
- A repeat DELETE → coded 404 `NOT_FOUND` (not a second 204, not a 500): delete is convergent, not idempotent-success.
