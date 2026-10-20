# CLI sweep 1124 — `Idempotency-Key` header blocked at the edge (live, 2026-10-20)

No fixture was created (verified by list). No new bug filed.

- A service create carrying an `Idempotency-Key` header → edge 403 on both attempts, and no service exists afterwards. The header is stopped before the application (same edge family as the method-override headers in sweep 1120), so header-based idempotency replay never reaches the API. Dedupe on retries is the operation-level semantics already covered in the idempotency sweeps (e.g. duplicate webhook create, deploy trigger), not a transport header.
