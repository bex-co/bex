# CLI sweep 640 — idle spin-down and wake lifecycle (live, 2026-10-19)

Disposable free web service `qa640-web` (`srv-dbbb5savfpec73dm0ipg`, hello-app image), `idleTTLSeconds=60` set via nested PATCH. Deleted and verified absent (REST 404, zero `qa640` services in list).

## Spin-down

- Service reached HTTP 200.
- With no further requests, instance count went 1 → 0 at ~113s after the last request (60s TTL plus sweep/reconcile cadence). Spin-down works.

## Wake

- First request after spin-down: `503 {"error":"service not ready","retryAfter":5}` in 2.06s — the activator signals retry instead of holding the request.
- Retry ~12s later: `503`; next retry ~12s later: `200` with the hello-app body. Instance count back to 1.
- Total wake latency from first request to 200: under ~35s. Reasonable cold-start behavior; the 503+`retryAfter` contract is explicit and retryable.

No bug filed.
