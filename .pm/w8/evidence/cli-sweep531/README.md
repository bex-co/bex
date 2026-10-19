# CLI sweep 531 — static-site serving, 404, and suspend behavior

**Verdict: no bug filed.** Same isolated session, `bex-canary`, HEAD build `218dffcd1`, 2026-10-19T09:58–09:59Z. One owned static site (public repo, root `examples/hello-go`, publish `.`) created → `live` in ~45 s → probed → suspended/resumed over REST → deleted by ID (former-URL `404`).

| step | observed result |
| --- | --- |
| Existing objects (`/render.yaml`, `/main.go`) | `200` with ETag, `cache-control: public, max-age=0, must-revalidate`; Range request → `206`. |
| `/` and missing paths (no `index.html` in this fixture) | Plain `404 "not found"` (`text/plain`, `nosniff`) for HTML and default Accept alike — no index exists for the ADR029 SPA fallback to serve, and extension-bearing misses stay 404 by design. |
| Suspend (REST) | `202`; host then answers the negotiated suspended `503` — `This service is suspended.` plain, `{"error":"service suspended"}` for JSON — with `Retry-After: 3600` and `Cache-Control: no-store`, over the managed certificate. Exactly the w3/m46 + w2/m98 contract in ADR029. |
| Resume | `202`; published bytes serve `200` again within ~10 s, no redeploy. |

## Safety and unrun checks

- Cleanup: exact-owned `srv-dbaujaiot0kc73chusk0` deleted by ID; former-URL 404 verified. Suspend/resume have no pinned-CLI verb (REST setup only).
- Unrun: SPA fallback with a real `index.html` (covered by sweeps 473/499-era static work), custom `404.html` (a recorded unbuilt future option in ADR029, not a defect), suspended-static behavior during the static-server poll gap (w4/307 territory, timing-sensitive).
- Deploy lag means deployed revisions can precede local `main`.
