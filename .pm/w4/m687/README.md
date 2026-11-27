# w4 · m687 — A web service gets the Cloudflare request headers a Render app gets, and a forged `CF-Connecting-IP` is refused 403 as on Render

**Worker:** worker4 **Goal:** an app moved from Render reads the client's address, country and request ray from the same `CF-*` headers on bex, and a public request that forges `CF-Connecting-IP` gets Render's `403 error code: 1000`. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's edge headers in `docs/render-artifacts/edge-headers.md` | 30m | — |
| [t002](t002.md) | Set `CF-Connecting-IP`, `CF-Visitor`, `CF-Ray` and `CDN-Loop` on requests from a non-Cloudflare peer | 1h | t001 |
| [t003](t003.md) | `CF-IPCountry` from a country table | 2h | t002 |
| [t004](t004.md) | A forged `CF-Connecting-IP` from a non-Cloudflare peer answers 403 `error code: 1000` | 45m | t002 |
| [t005](t005.md) | Responses carry `cf-ray`, and an uncached web-service response `cf-cache-status: DYNAMIC` | 45m | t002 |
| [t006](t006.md) | Amend ADR022 and ADR018, then verify on production with a disposable echo service | 45m | t003, t004, t005 |
| [t007](t007.md) | Render parity | 25m | t006 |
| [t008](t008.md) | Simplify | 15m | t007 |
| [t009](t009.md) | Test coverage | 30m | t007, t008 |
| [t010](t010.md) | Closeout | 10m | t009 |

Total: **10 tasks, about 7h05m**.

## Definition of done

- A request from a non-Cloudflare peer reaches a web service with `CF-Connecting-IP` (the client), `CF-IPCountry`, `CF-Ray`, `CF-Visitor` and `CDN-Loop` in the shapes Render's app receives (`docs/render-artifacts/edge-headers.md`); a caller's values for them never reach the app.
- A public request carrying its own `CF-Connecting-IP` from a non-Cloudflare peer gets `403` `text/plain` `error code: 1000`, byte for byte as on Render.
- A request from a tenant's own Cloudflare proxy keeps Cloudflare's values, as today.
- Responses carry `cf-ray` equal to the request's ray, and an uncached web-service response reads `cf-cache-status: DYNAMIC`; `server: cloudflare` is sent or its absence is a signed-off ADR018 divergence.
- ADR022's table and a new ADR018 row record the contract, and a disposable echo service on a named production pin shows it.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6`, 2026-11-26 (the aspect: what Render publishes outside its API pins that bex's parity ledger never read — its docs pages, its pricing page, and the Cloudflare headers its docs tell apps to use), filed to w4 by `/pm them to w4 and ship`. Re-checked at `origin/main` `f32c8164fb` before filing: no open, blocked or done item on any workstream owns it. Item 1.
- **Render:** every Render host sits behind Cloudflare, and Render's docs tell apps to use what it adds: [uptime-best-practices](https://render.com/docs/uptime-best-practices) ("Render includes this header in the request it sends along to your service", for `CF-Ray`) and the [PocketBase article](https://render.com/articles/host-pocketbase-on-render) (`trustedProxy.headers = ["CF-Connecting-IP"]`). Live captures from `httpbin.onrender.com` on 2026-11-27 are in t001.
- **bex today:** `deploy/gitops/charts/traefik-plugins/clientipheaders/clientipheaders.go`:88-95 removes every `CF-*` header from a non-Cloudflare peer (w8/m123's "absent"), so a Render-configured app reads no client address, no country and no ray. Tenant hosts are DNS-only, not behind Cloudflare ([w21/126](../../w21/blocked/126.md)).
- **Relation to w21/126:** that note waits for the user's decision on putting tenant hosts behind Cloudflare. This milestone does not wait for it: the plugin's trusted-Cloudflare-peer branch keeps Cloudflare's own values if that decision lands, and the parity rule says parity work is never parked on an optional decision.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first): an app moved from Render keeps its rate limits, audit logs and geo logic.
- **Expected outcome:** a PocketBase (or any) app configured as Render's docs say sees each user's real address on bex, and a support request can quote a `cf-ray` both sides log.
- **Why now:** Render's own articles now point apps at `CF-Connecting-IP`; w8/m123 chose "absent" without a live Render capture, which this milestone now has.
- **Render parity:** included: the edge contract is tenant-facing, though REST, GraphQL and MCP do not change.
