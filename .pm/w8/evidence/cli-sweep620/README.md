# CLI sweep 620 — datastore event streams do not exist; the 404 is uniform

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `c8717afdf`, 2026-10-19. Read-only probes; the baseline Postgres was only ever a URL path segment, never mutated. Completes sweep 618's Unrun item.

- `GET /v1/postgres/{baseline-id}/events` → `404 not found` — the route does not exist for datastores (service events live under `/v1/services/{id}/events`, sweep 618). The same 404 answers a synthetic `dpg-000…`, a synthetic `red-000…` under `/key-value/…/events`, and the baseline id addressed through `/services/{dpg-id}/events`.
- Because real and synthetic ids receive the identical response, the absence leaks nothing about which datastores exist.
- Query dialects are moot where there is no route: `limit=0` on the nonexistent path still answers the plain 404, not the pagination grammar.

## Unrun

- Whether datastore lifecycle history is exposed anywhere else (activity/audit feeds) — outside the CLI journey scope.
