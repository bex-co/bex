# ADR103: Security review — the static-site external-rewrite proxy

- **Status**: Accepted (2026-11-10, w3/m304)
- **Trigger**: w8/184 (2026-10-22) made a static-site rewrite to a public URL a server-side fetch from the platform's static-server, the tenant-controlled outbound request of the product. It shipped with "a dedicated review sign-off is still owed" (`.pm/w8/done/184.md`); this is that review.
- **Lineage**: a focused pass after [ADR085](ADR085-security-review-round23.md); the residuals it re-confirms (onbex.co PSL, ADR019's auth-only `:22`/`:6443`) are unchanged.

## Scope

`lego/operator/internal/staticserver/proxy.go` (`serveExternalRewrite`, `newExternalProxy`), its dialer `lego/types/netutil/ssrf.go` (`SafeDialContextFunc`, `UnsafeOriginIP`), destination validation `apps.validatePublicRewriteHost` (`lego/backend/internal/apps/service.go`) and the dashboard rule editor. Attacker: any tenant who owns a static site, which open signup makes anyone.

## Verdicts

| # | Attack | Verdict | Evidence |
| --- | --- | --- | --- |
| 1 | A destination host that resolves to a private, loopback, link-local or metadata address | Blocked | the dialer resolves, refuses any unsafe answer and dials the checked IP (`ssrf.go`); `TestExternalRewriteRefusesNonPublicDestinations` |
| 2 | DNS rebinding between check and connect | Blocked | the checked address is the one dialed |
| 3 | IPv4-mapped IPv6, NAT64, Teredo, 6to4 and other special-purpose prefixes | Blocked | `specialPurposePrefixes`; `net.IPNet.Contains` normalizes mapped answers |
| 4 | A literal non-public host in a saved rule | Refused at save | `validatePublicRewriteHost`, the dashboard validator |
| 5 | Following a redirect to an unvalidated second destination | Blocked | `CheckRedirect` returns `ErrUseLastResponse`; the 3xx is passed through |
| 6 | A rewrite into another tenant's static site on the same server | Blocked | own static hosts answer 508 before any dial |
| 7 | A loop through this server | Blocked | the `X-Bex-Static-Proxy` hop marker answers 508 |
| F1 | Platform-internal request headers reaching a third party | **Fixed** | the edge's `X-Forwarded-Server` (a Traefik pod name) was forwarded; it is now dropped. No secret-bearing internal header reaches the static-server. `TestExternalRewriteDropsPlatformInternalHeaders` |
| F2 | A loop through a service that drops the hop marker (site A → service B → site A …), or a site used as an open relay | **Fixed** | each hop held a goroutine and connection for up to 60 s with no bound. Proxied requests in flight are now capped at 256 per server and 32 per site (`fetchGate`); a site at its cap answers 503 with `Retry-After: 1` and leaves other sites alone. `TestExternalRewriteBoundsOneSitesProxiesInFlight` |
| F3 | A response over the 64 MiB budget | **Fixed** | it was cut silently under the destination's success status; the response is now aborted, so the visitor sees a broken transfer. `TestExternalRewriteAbortsAnOversizedResponse` |
| 8 | Visitor `Cookie`/`Authorization` forwarded to the destination | Accepted, intended | it is the purpose of the feature (a frontend reaching its API, Render's documented use). Cookies a browser sends to `*.onbex.co` scoped to `.onbex.co` are the #PSL residual (`.pm/DO_NOT_DO.md`), and any onbex.co web service already receives them |
| 9 | A destination's `Set-Cookie`, `Strict-Transport-Security`, `Alt-Svc` on the site's origin | Accepted | the tenant chose the destination and controls the origin's content anyway; `Alt-Svc` still needs a certificate for the site's host |
| 10 | The cluster's own node and load-balancer public IPs | Accepted residual | tenant pods are denied `host`/`remote-node`/`kube-apiserver` entities (`deploy/gitops/base/tenant-node-egress.yaml`), but the static-server is a platform pod and those addresses are public, so the dialer allows them. With no Hetzner firewall and no Cilium host policy, they are as reachable from the internet as from the proxy, and `:6443`/`:22` are ADR019's accepted auth-only baseline: the proxy grants nothing an internet client lacks. Re-open if a host firewall ever starts trusting cluster sources |
| 11 | Any public port and method (scanning or relaying through bex's address) | Accepted, bounded | Render documents "a full, publicly accessible URL" with no port rule; F2's cap bounds the volume |
| 12 | Bandwidth through the proxy escaping metering | Not a gap | static-site egress is counted from the Traefik router's response bytes (`usage.queryEgressBytesEvidence`), which proxied responses pass through |
| 13 | Request smuggling, hop-by-hop headers, slow destinations | Blocked | Go's client re-frames the request, `stripHopByHop` (incl. `Connection`-named headers), `ResponseHeaderTimeout` 30 s and a 60 s overall deadline |

## Sign-off

Reviewed and accepted on 2026-11-10 by the w3/m304 worker against `main`; F1–F3 ship with regression tests, items 8–11 are recorded residuals with the reason they stand. A production probe of the boundary is recorded in `.pm/w3/m304` (t004).
