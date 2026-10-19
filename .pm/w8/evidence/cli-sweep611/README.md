# CLI sweep 611 — service `port` PATCH dialects: both spellings, bounds, type safety

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `1a49cb806`, 2026-10-19. Owned fixture `qa611-web` (`srv-dbba6h2bpdss73ci0npg`, free hello-app, which honors `$PORT`), deleted at close: REST 404, URL 404, zero `qa611` names. REST boundary (the pinned CLI has no port flag).

- **Accepted and effective:** `PATCH {serviceDetails:{port:8080}}` returned 200; the service re-rolled with the new `$PORT` and kept answering 200, and reads showed `port: 8080`. The top-level spelling `{port: 9090}` was also accepted and effective — both coalesce to the same field, matching the patch handler's documented behavior.
- **Bounds refused with the reason:** `port: 80` and `port: 70000` both → `400 port must be 1024-65535 — a tenant container has no NET_BIND_SERVICE capability…`. (The explanation's tail speaks only of the low end; the range itself is stated correctly up front — cosmetic, not filed.) The service kept serving on the prior port after each refusal.
- **Type safety:** `port: "8080"` (string) → `400 request body contains an invalid value for field "serviceDetails.port"` — no silent coercion.

## Unrun

- Port PATCH on a static site or background worker (no public port semantics).
- Port PATCH while a deploy is in flight.
