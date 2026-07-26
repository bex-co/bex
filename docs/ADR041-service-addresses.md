# ADR041 — Service addresses: internal and public, for web and private services

**Status:** Proposed (2026-07-25)

The single reference for what a service's **public address** (the HTTPS origin the edge serves) and **internal address** (the private hostname sibling services connect to) look like on bex, what they look like on Render, and the decisions that close the gap. Datastore addresses (Postgres `dpg-…`, Key Value) have their own ADRs ([ADR009](ADR009-postgresql-management.md), [ADR021](ADR021-keyvalue-management.md)) and are out of scope here except where noted.

## Render's model (evidence)

| Axis | Render behavior | Evidence |
| --- | --- | --- |
| Public address | `https://<slug>.onrender.com`; `slug` defaults to the service name, silently gains a short random suffix (`-p9vq`, `-bkxk`) on a **global** collision | [duplicate-service-names.md](render-artifacts/duplicate-service-names.md) |
| Internal address | bare `<hostname>:<port>`, optional protocol prefix — `elasticsearch-2j3e:9200`, `http://elastic-qeqj:10000`. No DNS suffix, no namespace segment. The hostname is the **slug** (note the suffixed examples) | [render.com/docs/private-network](https://render.com/docs/private-network) |
| Which types are addressable | Web services and private services both carry an internal address (a private service labels it its **Service Address**); background workers and cron jobs can dial out but are not addressable | same |
| Scope | Same private network ⇔ same **region** and same **workspace**; project-environment network isolation narrows it further | same |
| Discovery | Dashboard **Connect → Internal** tab per service; the deploy-detail header also shows the internal address | same + [deploy-detail-page.md](render-artifacts/deploy-detail-page.md) (line "Render's header shows an internal address, while the current bex dashboard/API contract does not supply one") |
| Blueprint | `fromService: {property: host \| port \| hostport}` env vars resolve to the internal hostname/port | render.yaml docs |
| Default web port | `PORT` is injected; Render's default is `10000` (visible in the doc example `elastic-qeqj:10000`) | private-network doc |

## bex today

All tenant Apps share one namespace (`BEX_CP_APPS_NAMESPACE`, default `default`); isolation is label-scoped NetworkPolicy ([ADR022](ADR022-tenant-isolation.md)), not namespace or DNS scoping.

| Axis | bex behavior | Where |
| --- | --- | --- |
| Public address | `https://<slug>.<BEX_BASE_DOMAIN>`; slug = `spec.subdomain` (minted globally-unique with a random suffix on collision, w4/m19), fallback CR name; precedence `spec.host` → platform host → `spec.hosts[]` | `AppSpec.EffectiveHosts` / `PlatformSubdomain` (`lego/types/v1alpha1/app_types.go:511`), `apps.slug` column |
| Internal DNS identity | the k8s Service named after the **App CR**. Store-managed CRs are named `<tenant>-<name>` (`tea-<xid>-web`), so the resolvable internal hostname carries the tenant prefix | Service create `app_controller.go:1114`; `core.CRName` (`lego/backend/internal/core/base.go:728`) |
| Internal address surfaced | `status.URL` for a private service / worker-shaped App is the k8s FQDN `http://<crName>.<ns>.svc:<port>`; bex-api round-trips it into `serviceDetails.url`. No Connect-style internal-address surface exists | `app_controller.go:1241`; `apps/render.go` ("url" set whenever non-empty) |
| Port | `spec.port`, default **3000**; `PORT` injected, never user-shadowable | `app_controller.go:313`; `appEnv` |
| Blueprint | `fromService` `host`/`hostport` inject the **bare bex.yml name** (`api`, `api:8080`) | `apps/deploy.go:1426,1430`; asserted by `stack_test.go:242` |
| Scope | DNS resolution is cluster-wide; **reachability** is what's workspace-scoped (per-App NetworkPolicy; environment isolation narrows it, w6/m19) | [ADR022](ADR022-tenant-isolation.md) |

## Gaps

1. **`fromService` host literals appear broken for store-managed stacks.** The injected hostname is the bare bex.yml name, but since w4/m19 the Service object is named `<tenant>-<name>` — the bare name has nothing to resolve to. ([ADR006](ADR006-bex-api.md) §Blueprint still claims "bare `<name>` resolves because every bex Service is named after its App" — true only for legacy/hand-applied CRs whose CR name _is_ the bare name.) Needs a live repro, then a fix.
2. **The internal hostname is not Render-shaped.** Render: `<slug>:<port>`. bex: `<tenant>-<name>.default.svc:<port>` (tenant-prefixed CR name, k8s FQDN when surfaced).
3. **No internal-address surface.** Render shows Connect → Internal / Service Address / deploy-header internal address; bex's dashboard and API supply none (recorded in [deploy-detail-page.md](render-artifacts/deploy-detail-page.md) as deliberate scope at the time).
4. **A private service's `serviceDetails.url` leaks a k8s implementation detail** (`http://….svc:3000`) where Render's shape is unverified — capture what Render actually returns for a `pserv` before matching it.
5. **Default port differs** (3000 vs Render's 10000) — cosmetic; `PORT`-reading apps are unaffected.

## Decisions

### D1 — The internal hostname is the slug

The canonical internal address becomes **`<slug>:<port>`** — the same globally-unique slug the public host uses, so the two origins of one service differ only in suffix, exactly Render's shape:

```
public    https://<slug>.<BEX_BASE_DOMAIN>
internal  <slug>:<port>            (protocol prefix optional, chosen by the caller)
```

The slug is already globally unique across all workspaces (w4/m19), DNS-safe ([ADR020](ADR020-identifiers.md) hyphen rule), and **immutable** (service rename mutates only `displayName`, never `spec.subdomain` — ADR018 w8/m8 row), so slug-named Services cannot collide in the shared namespace and internal addresses are rename-stable. Legacy CRs without a slug fall back to the CR name via `PlatformSubdomain` — for them the slug-named Service coincides with today's CR-named Service, a no-op.

### D2 — Mechanism: a second slug-named ClusterIP Service

The operator adds a slug-named ClusterIP Service (same selector, same port, App-owned) alongside the existing CR-named Service for every addressable App (web + private). The CR-named Service **stays** — existing in-cluster consumers, `status.URL` readers, and hand-applied CRs keep working; the slug-named Service is the documented, surfaced identity. Bare `<slug>` resolves in-pod via the namespace search path (`default.svc.cluster.local`), giving Render's suffix-less form without touching DNS config.

Isolation is unaffected: NetworkPolicies select **pods** by label ([ADR022](ADR022-tenant-isolation.md)); a second Service over the same pods changes nothing. As on Render-with-a-twist: cross-workspace names still _resolve_ (shared cluster DNS) but the connection is **denied** by the workspace policy — resolution is not access control; reachability is, and that boundary already holds.

### D3 — `fromService` resolves to the slug

`host` → `<slug>`, `hostport` → `<slug>:<port>` (`port` unchanged). This fixes gap 1 (the literal finally names a real Service) and lands parity in the same move. Requires create-path plumbing so sibling slugs are known at resolve time (slugs are minted at create).

### D4 — Surface the internal address on all surfaces

Following the `sshAddress` precedent ([ADR035](ADR035-ssh.md)): REST/GraphQL/MCP expose an internal-address field for web + private services, and the dashboard grows a Connect-style affordance (Internal tab; a private service shows it as its Service Address; deploy-detail header). **Gated on a live capture** of Render's exact REST field names for `pserv` and `web` serviceDetails (gap 4) — bex matches the captured shape rather than inventing one; until then `serviceDetails.url` keeps its current value to avoid churning twice.

### D5 — Conscious divergences (kept)

| Divergence | Rationale |
| --- | --- |
| Default `PORT` stays **3000** (Render: 10000) | changing it rolls every existing tenant Deployment for a cosmetic default; `PORT`-reading apps see no difference |
| One addressable port per service (Render private services may listen on many) | the k8s Service exposes `spec.port` only; pod-to-pod same-workspace traffic is already port-unrestricted, so a second port works by pod IP but is not part of the addressable contract |
| No region axis | bex is one cluster; the private-network scope is workspace (+ environment isolation), never region |
| Cross-workspace DNS **resolution** succeeds (connection denied) | shared cluster DNS; the enforced boundary is NetworkPolicy reachability, verified by `make verify-tenant-isolation` |

## Consequences

- One more Service object per addressable App (slug-named), owned + garbage-collected via the App CR like every sibling resource.
- `status.URL` for private services should migrate to `http://<slug>:<port>` once D2 lands (the FQDN keeps working; the surfaced string becomes Render-shaped). Blueprint docs in [ADR006](ADR006-bex-api.md) §`fromService` need the stale bare-name claim corrected.
- The parity ledger ([ADR018](ADR018-render-parity.md)) gains address rows (internal address surface, Connect UX) once D4 ships.

## Rejected options

- **Rename the workload Service to the slug in place** — breaks every existing consumer of the CR-named Service and churns `status.URL` for all Apps at once; the dual-Service shape is strictly safer and the old name can be retired later if ever.
- **Bare user-name Service** (`web`) — collides across workspaces in the shared namespace; this is exactly why w4/m19 prefixed CR names. The slug solves the same problem with a Render-shaped answer.
- **Namespace-per-workspace DNS scoping** (so bare names resolve per-tenant) — re-litigates ADR022's Option A; rejected there for projector churn and blast radius, unchanged here.
- **Per-instance DNS records** (Render resolves an internal hostname to individual instance IPs) — would need a headless Service and per-pod addressing semantics; deferred until something needs instance-level dialing (the SSH gateway already covers instance targeting).
