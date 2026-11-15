# w21 · m417 — Public buckets: `dev-url` and objectgw serve a bucket read-only at `pub-<obk-id>.onbex.co`, with caps, an egress alert and a kill switch

**Worker:** worker21 **Goal:** a workspace admin publishes a bucket with `wrangler r2 bucket dev-url enable` and anyone can read its objects over the web without a credential, bounded so it cannot take down the shared Wasabi account. **Status:** blocked — every implementable task (t001–t007 and t009–t011) is done; t008 (the production walk) and t012 (closeout) need the production cluster, which w21/m415 t010 and w21/m416 t013 must walk first (see t008's gate)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | `dev-url` routes, the `pub-` slug reservation and the host lookup route for objectgw — **DONE** | 50m | w21/m416/t001, w21/m416/t002 |
| [t002](done/t002.md) | objectgw core: host lookup, GET and HEAD with `Range` and conditional requests, 404 for everything else — **DONE** | 60m | t001, w21/m415/t003 |
| [t003](done/t003.md) | objectgw headers: `nosniff`, CORS with preflight from the bucket's rules, a bounded cache and byte counting — **DONE** | 50m | t002, w21/m416/t005 |
| [t004](done/t004.md) | Rate and bandwidth caps and the month-to-date public egress alert — **DONE** | 50m | t003 |
| [t005](done/t005.md) | Deploy objectgw: image, manifests, the read-only Secret, and the kill-switch runbook — **DONE** | 55m | t002 |
| [t006](done/t006.md) | Route `pub-*` hosts to objectgw through Traefik and check the wildcard certificate covers them — **DONE** | 45m | t005 |
| [t007](done/t007.md) | Public read tests: private, unknown, `Range`, CORS, listing and keys — **DONE** | 45m | t003, t004, t006 |
| [t008](t008.md) | Production walk for public reads: enable `dev-url`, read with `Range` and CORS, disable, and measure the kill switch | 50m | t006, t007, w21/m416/t013 |
| [t009](done/t009.md) | Render parity (Cloudflare R2 reference; Render has no public buckets) — **DONE** | 30m | t007 |
| [t010](done/t010.md) | Simplify — **DONE** | 20m | t009 |
| [t011](done/t011.md) | Test coverage — **DONE** | 45m | t009, t010 |
| [t012](t012.md) | Closeout | 10m | t008, t011 |

## Definition of done

1. `dev-url enable` on a bucket makes its objects readable at `pub-<obk-xid>.<BEX_BASE_DOMAIN>` with `Range`, conditional requests and the bucket's CORS rules; `disable` stops it; private is the default.
2. The gateway cannot list, write or read a private or deleted bucket, and a service cannot take a `pub-` host.
3. Per-bucket and per-workspace rate and bandwidth caps and an egress alert (with its panel) exist, and scaling objectgw to zero is a documented and measured kill switch.
4. Production shows a public object read with `Range` and CORS, a disable that takes effect, and the measured kill-switch answer.

## Source + Goal linkage

- **Source:** [ADR112](../../../../docs/ADR112-object-storage.md), filed by `/pmx adr 112 to w21` on 2026-11-15; ADR112 tasks t006 and t009 (public part), reorganized into three milestones.
- **Goal linkage:** ADR008 Render parity first, and the user's 2026-11-14 goal: serve what customers need from objects; Render has no public access, Cloudflare R2 does, so bex follows R2's `dev-url`.
- **Expected outcome:** A customer hosts public assets (images, downloads) from a bucket without running a server, and the platform can switch it off.
- **Why now:** Public reads are the case most likely to break Wasabi's egress fair use on the shared account, so the caps, alert and kill switch land with the feature, after the store and wrangler contract exist.
- **Render parity task:** included. `dev-url` is a customer-facing REST surface and the `pub-` host a customer-facing behavior; the reference is Cloudflare R2 (Render has none).

## Parked (2026-11-15)

Everything that can be built, tested and checked offline is done: `dev-url`, objectgw, its caps and counting, the egress gauge and alerts, the manifests, the routes, the kill-switch runbook, the docs and the parity comparison (t009 no longer waits for t008, as in m415 and m416, because it needs no production). What remains needs the production cluster: **t008** and so **t012**. Before the walk, `scripts/object-storage-s3-credentials.sh provision` must run once more (it keeps the Wasabi pairs and mints `BEX_OBJECT_STORAGE_GATEWAY_TOKEN`) and `install` must write both Secrets. The walk also resets the first-value caps and the two alert thresholds and replaces the expected Traefik answer with a measured one. Needs: the user's read access to the production cluster (`/unblock w21`).

## Open decisions

- User-changeable, recommendation recorded: the public host lives on `onbex.co` (ADR112 D4); it shares a registrable domain with tenant apps, the same risk class as static sites.
- Recommendation, not a decision: the kill switch is scaling objectgw to zero; an exact-404 switch would need a runtime flag store the backend does not have.
- Not filed: custom domains, a CDN and signed public URLs (ADR112 D4 and D10).
