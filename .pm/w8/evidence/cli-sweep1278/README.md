# CLI sweep 1278 — static routes and headers in real `render.yaml` files (read-only, 2026-10-21)

Production `https://api.bex.co/v1/`, pin `9cace6b6e25d`, HEAD-built pinned CLI. Read-only validation; no fixtures.

Across the sweep 1270 corpus (601 files): 92 route rules (84 rewrite, 8 redirect). The most common sources are `/*` (66) and `/api/*` (8). 19 rules have absolute destinations: 6 redirects to a CDN and 13 rewrites to `https://<backend>.onrender.com/…`. The most common headers are `Cache-Control` 40, `X-Frame-Options` 24, `X-Content-Type-Options` 17 and `X-Robots-Tag` 13.

- `redir.yaml`, two absolute-URL redirects → `routes[0].destination must be a path starting with /`. Filed as [w8/162](../../162.md), which asks to split redirects from rewrites.
- `rewrite-ext.yaml`, an `/api/*` → `https://api.example.com/api/*` rewrite → same refusal. This is the recorded local-only divergence (ADR018:211, ADR029:50) and is not filed.
- Also noted: Render's Node docs use `18.18.0` as an example of a pinnable version, while bex refuses Node 18 by name (ADR060, deliberate; seen live in sweep 1277's `fastify-hello-world`).
