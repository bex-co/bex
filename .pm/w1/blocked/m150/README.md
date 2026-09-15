# w1 · m150 — A service's inbound IP allowlist matches the load balancer's private address, not the client

**Worker:** worker1 **Goal:** an inbound IP allowlist on a web service or static site admits exactly the public clients whose address falls in a listed CIDR, and nobody else. The address Traefik matches is the client's, carried through the Hetzner load balancer by PROXY protocol and trusted only from that load balancer, the way `w2/done/m57` t010 already does for Postgres and Key Value. No header a client sends can influence the match. **Status:** blocked (2026-09-15). Needs your judgement; see § Blocked.

## Blocked — needs your judgement (2026-09-15)

Triaged on `main` at `5523f684e`: the bug is still real and nothing has fixed it.

- `database_controller.go:515` emits `ipAllowList.sourceRange` with no `ipStrategy`.
- `infra/terraform/main.tf:208,226` keep `proxyprotocol = false` on `http` and `https`.
- `traefik.values.yaml` has no `proxyProtocol`.

The engineering is clear:

1. Traefik trusts PROXY protocol from `10.10.0.7/32` only.
2. After a probe shows (1) is live, flip the load balancer's `http`/`https` listeners.

It was **not** started, because each of these is yours to decide.

1. **May we change the production edge?** Enabling PROXY protocol on the Hetzner load balancer's :80/:443 listeners changes every HTTP(S) request to every tenant. If the listeners flip before Traefik expects PROXY headers, or Traefik rolls back after, the result is a full HTTP(S) outage. Nothing can confirm the ordering today: Traefik rolls through Argo, the production runner has no kubectl, and `infra.yml` waits only on the datastore proxies' deploy. Do you approve the two-step rollout (Traefik first, verified by a live probe, then the Terraform listener flip, each a separate ship), and who applies the Terraform change?
2. **What should an allowlist mean behind Cloudflare?** `api.bex.co`, `dashboard.bex.co` and any Cloudflare-proxied tenant custom domain reach Traefik from a Cloudflare edge address. PROXY protocol fixes the Hetzner hop, but an allowlist on those hosts would still match Cloudflare, not the client. Choose one:
   - (a) accept it and document that allowlists only work on DNS-only (grey-cloud) hosts;
   - (b) additionally trust `CF-Connecting-IP`, but only when the peer is in Cloudflare's published ranges (a second trusted-proxy list to keep current);
   - (c) refuse or warn when an allowlist is set on a Cloudflare-proxied host.

Answer both and the milestone can move back to `w1/m150/` and proceed in task order.

## Tasks (in order)

| id   | title                                                                                                                  | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Traefik `web`/`websecure` accept PROXY protocol only from the load balancer (`10.10.0.7/32`), rolled out before the listener change | 45m | —          |
| t002 | Enable PROXY protocol on the Terraform `http`/`https` listeners and prove the allowlist sees the client               | 45m | t001       |
| t003 | Blast radius: every consumer of Traefik's client address on :80/:443                                                   | 45m | t001       |
| t004 | Render parity                                                                                                          | 20m | t002, t003 |
| t005 | Simplify                                                                                                               | 15m | t004       |
| t006 | Test coverage                                                                                                          | 40m | t004       |
| t007 | Closeout                                                                                                               | 10m | t006       |

## Definition of done

Each bullet can be repeated on a throwaway free web service (`bex-co/bex` `examples/hello-go`, docker) that is live and returning `200`. Run it from a machine outside the cluster whose public IPv4 is `<me>` (`curl -4 -sS https://dashboard.bex.co/cdn-cgi/trace | grep ^ip=`). Set entries with `setServiceIpAllowList` (or Settings → **Networking**), then poll `curl -4 -sS -o /dev/null -w '%{http_code}' https://<svc>.onbex.co/` for about 30s. Only states observed at filing time are listed:

- **Your own address is admitted.** Entries `[<me>/32]` → `200`. At filing time the response was **`403 Forbidden`** from 23s after the save until the entry was changed.
- **The load balancer's address admits nobody outside.** Entries `[10.10.0.7/32]` → `403` from `<me>`. At filing time the response was **`200`**.
- **An empty list stays open.** Entries `[]` → `200`. That passed at filing time, and this milestone must keep it.
- **The request log records the client.** `GET /v1/logs?resource=<srv>&type=request&limit=50&direction=backward` returns records whose `ClientHost` is `<me>` for the probe requests. At filing time all 50 of 50 records read `ClientHost: 10.10.0.7`.

## Evidence (probes run 2026-09-14, production, workspace `bex` / `tea-d98210cbbpdc73dcrkvg`)

Fixture: free web service `qa-20260914-ip` (`srv-dajukjggsm7s73f64d60`). It was live and its URL returned `200` at 12:39:18Z. It was deleted inside the run: `DELETE /v1/services/…` returned `204`, then `GET` returned `404`, and the URL returned `404` at 12:47:00Z.

1. **Before any allowlist.** `GET /v1/logs?resource=srv-dajukjggsm7s73f64d60&type=request&limit=50&direction=backward` returned 50 Traefik access-log records with a single distinct `ClientHost`, `10.10.0.7` (sample `ClientAddr` `10.10.0.7:54626`). The records carry no request headers: the keys are `ClientAddr`, `ClientHost`, `ClientPort`, `Downstream*`, `Request*`, `Router*`, `Service*` and similar.
2. **Caller.** `curl -4 https://dashboard.bex.co/cdn-cgi/trace` gave `ip=162.224.81.143`. The fixture host resolves to A `49.12.20.236` only (no AAAA), and `curl` connected via `49.12.20.236`.
3. **Allowlist probes** (GraphQL `setServiceIpAllowList(id, entries)` from the signed-in dashboard page; `curl -4` from `162.224.81.143`):

   ```text
   12:43:28 entries=[{cidrBlock:"162.224.81.143/32"}] → 200, ipAllowListEntries echoes the entry
   12:43:51 curl → 403 body "Forbidden" (0.51s); still 403 at 12:44:08
   12:44:42 entries=[{cidrBlock:"10.10.0.7/32"}]      → 200
   12:45:03 curl → 200 (first sample after the save); still 200 at 12:45:14
   12:45:39 entries=[]                                → 200, ipAllowListEntries: []
   12:46:03 curl → 200
   ```

4. **Traefik's contract** (`doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/ipallowlist/`, fetched 2026-09-14): with no `ipStrategy`, the middleware matches `sourceRange` against "the Remote address found in the request". `ipStrategy.depth` and `excludedIPs` read `X-Forwarded-For` instead. The default reject status is `403`.
5. **Render's contract** (`render.com/docs/inbound-ip-rules`, fetched 2026-09-14): "Disallowed IPs are automatically blocked with a `403 Forbidden` response." Rules are available for web services, static sites, environments and workspaces on Scale/Enterprise, and the default is `0.0.0.0/0`. The page does not say which address is evaluated.
6. **The dashboard's promise.** Settings → Networking card: "Restrict inbound HTTP traffic to these source CIDRs." (`dashboard/src/features/services/locales/en.ts:3945-3947`).

No screenshots were taken; the transcripts above are the evidence.

## Root cause

- **The middleware matches the TCP peer.** `cidrMiddlewareSpec` (`lego/operator/internal/controller/database_controller.go:510-516`) emits `{"ipAllowList": {"sourceRange": …}}` with no `ipStrategy`, so Traefik matches the request's remote address (Evidence 4).
- **That peer is the load balancer.**
  - The Hetzner `http` and `https` listeners set `proxyprotocol = false` (`infra/terraform/main.tf:201-235`).
  - The Traefik Service's `externalTrafficPolicy: Local` (`deploy/gitops/overlays/prod/values/traefik.values.yaml:66`) preserves only the load balancer→node source.
  - The `web` and `websecure` ports (`:43-48`) configure no `proxyProtocol`.
  - `scripts/gitops-validate.sh:168-173` pins `proxyprotocol=false` on every edge listener except `postgres` and `valkey`.

  Traefik therefore sees every Internet client as `10.10.0.7` (Evidence 1). A real client CIDR never matches, and an entry covering `10.10.0.7` matches everyone (Evidence 3).
- **Why it shipped green.**
  - `w2/done/m57/done/t010.md` recorded this mechanism: "Hetzner TCP load balancers do not preserve an application-visible client address unless PROXY protocol is enabled". It fixed it for the two datastore listeners and put "enabling PROXY protocol on SSH or HTTP listeners" out of scope.
  - `w7/done/m32`'s DoD was "Verified live against the mock cluster from two source IPs". The kind/CAPD mock has no Hetzner load balancer in the path.
- **`ipStrategy` is not the fix.** The load balancer is layer 4 and adds no `X-Forwarded-For`. Trusting a forwarded header here would let any client name its own address.

## Blast radius

- **Allowlist producers.**
  - `cidrMiddlewareSpec` has one caller (`app_controller.go:2816`), inside `reconcileIPAllowListMiddleware`. That function emits both layers: the service's `-ip-allow` and the environment's `-env-ip-allow` (`w4/m28`).
  - It has two callers: `reconcileIngressWithMiddlewares` (`:2354`, services with a host, including the activator-fronted path) and the static-site path (`:3166`).
  - `sourceRange` is produced only at `database_controller.go:515`.
  - No platform middleware under `deploy/` or `infra/` uses `ipAllowList` (0 hits), so no platform allowlist changes meaning.
- **The fix is global, not allowlisted.** PROXY protocol on :80/:443 changes the client address for **every** HTTP host, not only allowlisted ones:
  - the access-log `ClientHost` (request logs);
  - the `X-Forwarded-For` Traefik appends for every backend: tenant apps, the activator, static-server, bex-api;
  - bex-api's IP-keyed rate limiters (`BEX_TRUSTED_PROXY_CIDRS=10.244.0.0/16`, `lego/operator/config/api/deployment.yaml:594`; `lego/backend/internal/core/clientip.go:90-117`).
- **Outage risk.** A load balancer that sends PROXY headers to an entrypoint that does not expect them breaks all production HTTP(S). t001 must be live before t002 applies, the same compatibility-first order `w2/m57` t010 used. Hetzner's TCP health checks on `31218` and `31976` must stay green.

## Adjacent classes

- **Denied** stays `403` (Traefik's default, matching Render's documented response).
- **Empty list** stays open. An explicitly empty environment rule keeps projecting `DenyAllCIDR` `255.255.255.255/32` (`lego/backend/internal/core/cidr.go:188-193`) and keeps denying everyone.
- **Spoofing.** A client-sent `X-Forwarded-For` or PROXY header from any peer other than `10.10.0.7/32` must never influence the match.
- **Postgres and Key Value** are already correct via `w2/m57` t010 and stay unchanged.
- **SSH** keeps its own PROXY chain (`w4/m82`, `w6/m132`) and stays unchanged; its listener remains `proxyprotocol = false`.
- **Cloudflare-fronted hosts** (`api.bex.co`, `dashboard.bex.co`, and any tenant custom domain proxied by Cloudflare): after the fix, Traefik's peer is a Cloudflare edge address, not the end user. t003 decides whether that is accepted or needs `forwardedHeaders.trustedIPs` for Cloudflare's ranges, and records the decision.

## Unverified (reasoned, not probed this run)

- **One vantage point.** Only one outside address (`162.224.81.143`) was probed. That every external client appears as `10.10.0.7` rests on 50 of 50 log records; whether the load balancer ever uses another private address was not checked.
- **Wider private entries.** A tenant entry such as `10.0.0.0/8` admitting the whole Internet is inferred from the `10.10.0.7/32` probe, not probed.
- **Other surfaces.** Static sites, environment-layer rules and custom domains were not probed. They share the middleware and Ingress (code-read only).
- **bex-api's rate-limit keys.** Whether bex-api's rate limiters today key every anonymous client as `10.10.0.7` (through Traefik's `X-Forwarded-For`) was not probed. If true, that is a separate finding; t003 checks it.
- **Traefik without a header.** Traefik's handling of a trusted peer that sends **no** PROXY header was not checked against the pinned image, and the compatibility-first order depends on it (t001).
- **Render.** Which address Render evaluates is not documented.

## Dedupe

- `w7/done/m32` shipped the feature and `w9/done/m56` the structured entries. Both were verified without a Hetzner load balancer in the path. The allowlist never worked on production, so this is filed as a new defect, not a regression.
- `w2/done/m57` t010 fixed the datastore listeners and explicitly excluded HTTP. `w6/done/m132` covers the SSH PROXY-protocol mismatch, a different listener.
- No open item mentions PROXY protocol, the client address, `externalTrafficPolicy` or `10.10.0.7`. The only grep hit outside `done/` is the closed `w4/m82` line in `.pm/w4/README.md`.
- `.pm/DO_NOT_DO.md`: no conflict. Line 19 forbids a static source-IP firewall on `:22`/`:6443`; tenant HTTP allowlists are unrelated.
- Not fixed on `main` as of `f0dcab94a`: `git log -S"ipStrategy"` and `-S"forwardedHeaders"` show no fix, and `main.tf` still sets `proxyprotocol = false` on `http` and `https`.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 2026-09-14 pass 11: web service Settings → Networking inbound IP allowlist (`docs/ADR018-render-parity.md` row 125, `w7/done/m32`, `w9/done/m56`).
- **Goal linkage:** pillar 1, Render-compatible hosting (`docs/ADR018-render-parity.md` row 125 marks it ✅ on every surface), and network access control that does what it says.
- **Expected outcome:** the Networking card's "Restrict inbound HTTP traffic to these source CIDRs" becomes true. Listing your office IP admits your office and nobody else, and request logs show real client addresses.
- **Why now:** the feature fails silently in both directions:
  - A tenant who restricts a staging service to their own IP locks themselves out with a bare `403` and no hint why.
  - A tenant who lists a private range, or the one entry that "works", believes the service is restricted while it is open to the Internet.

  Fixing it changes the edge for all HTTP traffic, so it needs a planned, ordered rollout rather than an incidental change.
- **Render parity:** included. No REST, GraphQL, MCP or dashboard shape changes, but the user-visible behavior of a ✅ Render-parity row changes. t004 records the address semantics and the Cloudflare caveat on row 125.
