# CLI sweep 906 — response-header surface (live, 2026-10-20)

Read-only probes (baseline read only). No new bug filed.

- Ordinary JSON reads carry `Content-Type: application/json` and no `ETag`; a bogus `If-None-Match` is ignored (200, full body) — the REST surface does not do conditional requests, so clients cannot be stranded on stale validators. The one deliberate cache directive in this area remains the deploy-hook's `no-store` (sweep 892).
