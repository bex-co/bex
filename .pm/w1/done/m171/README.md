# w1 · m171 — Warn when an inbound IP allowlist sits behind a Cloudflare-proxied custom domain

**Worker:** worker1 **Goal:** a tenant who saves an inbound IP allowlist on a service, or on an environment, whose custom domain is proxied by Cloudflare is told on every surface that the allowlist sees Cloudflare's address there, not the client's. The save still succeeds. **Status:** done 2026-10-25. Every Definition of done item was observed live in production on 2026-10-25 without a user-owned DNS zone (§ Live verification). The change has been deployed since 2026-10-02 (`1263d12ae`).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Shared Cloudflare-proxied host detection in `core` — **DONE** | 35m | — |
| t002 | Service allowlist warning on REST, GraphQL and MCP — **DONE** | 45m | t001 |
| t003 | Environment allowlist warning on REST, GraphQL and MCP — **DONE** | 40m | t001 |
| t004 | Dashboard notices: service Networking card + environment settings (en/zh) — **DONE** | 40m | t002, t003 |
| t005 | Render parity — **DONE** | 15m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 40m | t005 |
| t008 | Closeout (deploy + live verify) — **DONE** | 30m | t006, t007 |

## Definition of done

Each item is a state you can observe in production:

1. **Proxied host is named.** On a web service with an orange-cloud custom domain and a restricting allowlist (for example `203.0.113.0/24`):
   - `PATCH /v1/services/{id}` with `serviceDetails.ipAllowList` answers `200`, and its `serviceDetails.ipAllowListProxiedDomains` names that host.
   - `GET /v1/services/{id}` names it the same way.
   - GraphQL `service(id){ipAllowListProxiedDomains}` and `setServiceIpAllowList{ipAllowListProxiedDomains}` name it.
   - MCP `get_service` and `update_service` return a second text block that names it and says "Cloudflare".
2. **Grey-cloud and platform hosts are not named.** A DNS-only custom domain and the `<slug>.onbex.co` host never appear in the field.
3. **No restriction, no warning.** With the allowlist cleared, or containing `0.0.0.0/0` or `::/0`, the field is absent from REST and MCP prints only the JSON block.
4. **Environment layer.** An environment with a restricting allowlist and a member that has a proxied custom domain names that host on:
   - REST `GET`/`PATCH /v1/environments/{id}`;
   - GraphQL `environments(projectId){ipAllowListProxiedDomains}`;
   - MCP `get_environment`/`update_environment`.
5. **Dashboard.** At desktop and narrow-mobile widths, the service Settings → Networking card and the environment settings dialog both show the notice with the host. The notice is pluralized and present in en and zh, and the list still saves.

## Live verification (2026-10-25)

Run in production as the QA user, through the dashboard session (REST, GraphQL and MCP called from the page). All times are UTC.

**How the fixture avoids a user-owned zone.** The 2026-10-02 blocker no longer holds, for two reasons:

- Custom domains now verify by their traffic DNS, not a `_bex-challenge` TXT (w5/m137, `a8c215fdf`).
- Every `*.bex.co` host is reserved as a platform apex (w4/190), so a `bex.co` sub-label could never have been claimed.

The fixture therefore used two sslip.io names:

- **P = `2606-4700-3033--ac43-8a88.49-12-20-236.sslip.io`** resolves to A `49.12.20.236`, the bex edge, and AAAA `2606:4700:3033::ac43:8a88`, which is inside Cloudflare's `2606:4700::/32`. It verifies, because one of its addresses is the platform host's address. The detector names it because one of its records is in a Cloudflare range.
- **G = `49-12-20-236.sslip.io`** resolves only to the edge: a verified DNS-only host.

P exercises the deployed detector on real public DNS. It is not a real orange-cloud record, which would answer only Cloudflare addresses. The detector treats both alike because any one record in range suffices, and a real orange-cloud host is verified while it is DNS-only, before the proxy is switched on.

**Plan gate.** The first fixture, in `bex-canary`, was refused with `ipAllowList: inbound IP rules for web services, static sites and environments require the Scale plan or higher`. That is the w8/m138 plan gate, shipped 2026-10-23, and correct, because `bex-canary` is on Hobby. The fixture was deleted (`204` → `404`) and recreated in the `bex` workspace (Scale) as project `prj-dbf9818kbcjs73a5r720` (environment `evm-dbf9818kbcjs73a5r72g`) with free image `web_service` `srv-dbf981910mgc73d98ptg`. Both domains were added (`201`) and verified (`202`, then `verified`).

Results:

1. **Proxied host is named** (23:43:15Z). The allowlist was `203.0.113.0/24`.
   - REST `PATCH /v1/services/{id}` answered `200` with `serviceDetails.ipAllowListProxiedDomains = [P]`, and `GET` returned the same.
   - GraphQL `service(id){ipAllowListProxiedDomains}` and `setServiceIpAllowList{…}` both returned `[P]`.
   - MCP `update_service` and `get_service` (23:43:30Z) each returned a second text block: "Warning: these custom domains are proxied by Cloudflare, so on them the inbound IP allowlist matches Cloudflare's edge address instead of the client's: P. Set their DNS records to DNS-only for the allowlist to see real client addresses."
2. **Grey-cloud and platform hosts are not named.** No surface ever listed G or `qa-20261025-m171-allow.onbex.co`.
3. **No restriction, no warning.**
   - With `0.0.0.0/0`, and again with the list cleared, REST `PATCH` and `GET` (`200`) omitted the field and GraphQL returned `[]`.
   - MCP `update_service` with a cleared list, and the following `get_service`, returned only the JSON block (23:43:40Z).
4. **Environment layer** (23:43:40Z). The environment allowlist was `203.0.113.0/24`.
   - REST `PATCH` and `GET /v1/environments/{id}` (`200`) returned `ipAllowListProxiedDomains = [P]`.
   - GraphQL `environments(projectId){ipAllowListProxiedDomains}` returned `[P]`.
   - MCP `get_environment` and `update_environment` each returned the same warning block.
5. **Dashboard.** Both the service Settings → Networking card and the environment "All settings" dialog show "The allowlist can't see clients on 1 custom domain", naming P and not G.
   - Screenshots at 1440×900 and 390×844: [`evidence/`](evidence/). The ready state is `m171-{service,env}-{desktop,mobile}-ready.png`.
   - The pending state (`*-pending.png`) was captured with GraphQL delayed 4s. The page is server-rendered, and the notice is conditional on data rather than a reserved region, so no skeleton geometry is involved.
   - The zh copy is covered by t004's locale tests; it was not switched live.

**Cleanup** (23:45:42Z): service `204` → `404`; project `204` → `404`; environment `404`. The `bex-canary` fixture `srv-dbf972gkbcjs73a5r6rg` / `prj-dbf9710kbcjs73a5r6pg` was deleted earlier, `204` → `404`. No DNS records were created, and the QA session was logged out.

## Triage (2026-10-02, `/loopx w1`)

**Verdict: Work.** The premise holds, and Decision 2 already settled the open question.

- **Nothing was built.** `grep -ri cloudflare lego/backend/internal dashboard/src` before this change found only the certificate-reason comment in `apps/domains.go` and the cert fix-it locale string. No allowlist path inspected the hosts behind it.
- **The form is decided.** m150 § Decisions 2 says to refuse, or warn where refusing would break a config that already exists. Refusing now breaks every tenant who already saved such an allowlist, so this milestone takes the **warn** form. That needs no further user decision.
- **Sized as a milestone.** It touches backend detection, two features on three surfaces, and the dashboard: more than an hour, and more than one task, so `w1/119` was promoted.

## Decisions

1. **Detection is a resolver check against Cloudflare's published ranges.** It does not reuse `domainCertificateReason`.
   - **Why not the certificate reason.** The w3/037 code is not a proxied-host classifier. It relays cert-manager's free-text reason from the Challenge, the Order or the Certificate's `Ready` condition, and it is empty once a certificate is issued. That is the steady state for a proxied host whose certificate was issued before it went orange, or whose HTTP-01 challenge Cloudflare passed through. Reusing it would miss most proxied hosts and would flag unrelated ACME failures.
   - **How the check works.** `core.ProxiedHostDetector` resolves each custom domain (`spec.host` + `spec.hosts`) with a 2s bound and a 5-minute per-host cache. A host counts as proxied when any of its A or AAAA records falls inside Cloudflare's ranges, which are embedded and were fetched 2026-10-02 from `cloudflare.com/ips-v4` and `/ips-v6`. The `w1/119` note itself named this alternative.
   - **Why a fixed list is acceptable here.** Decision 2 rejected option (b) because it adds a second trusted-proxy list somebody has to keep current. That objection applies to trusting the list for enforcement. Here a stale list can only miss a warning; it never admits or refuses traffic.
2. **Warn only while the list restricts.** The field is empty for:
   - an empty service list (open);
   - an empty environment list (deny-all, which blocks Cloudflare and clients alike);
   - any list containing a `/0` prefix (Render's seeded allow-all).
3. **Service field cost follows the `pushDeliveryMethod` precedent.** Only `Get` (REST by-id GET, GraphQL `service`/`server`, MCP `get_service`) and the allowlist writes compute it. Service lists do not. Environment views compute it everywhere, because the dashboard reads environments through the project list and one App list serves the whole page.
4. **A failed lookup degrades to "nothing to warn about".** It never fails the read or the save.

## Implementation notes

- **Backend.**
  - `internal/core/proxiedhosts.go`: the detector, `AllowListRestricts`, `AppCustomDomains` and the MCP sentence.
  - `apps`:
    - `AppView.IPAllowListProxiedDomains`;
    - `SetIPAllowList` and `Get` fill it;
    - `ApplyServicePatch` carries it past later patch rows;
    - REST emits it at `serviceDetails.ipAllowListProxiedDomains`;
    - GraphQL `Service.ipAllowListProxiedDomains`;
    - `renderServiceResult` adds the MCP text block.
  - `environments`: `EnvironmentView.IPAllowListProxiedDomains`, filled in `view`, `List` and `ListWorkspace`, plus the REST, GraphQL and MCP equivalents.
  - `mcputil.WithWarning`.
  - Wired through `api.Deps.ProxiedHosts` from `cmd/api`. Tests leave it nil, so no test touches real DNS.
- **Dashboard.**
  - `common/components/ip-allow-list-proxied-notice.tsx`, with pluralized `common.ipAllowListProxied*` keys in en and zh.
  - The notice appears in `ServiceNetworkingPanel` and `EnvironmentSettingsDialog`.
  - GraphQL selections: `server.graphql`, `ip-allow-list.graphql` and `EnvironmentFields`. Codegen ran offline from `TestDumpGraphQLSchema`; every hunk is feature-only.
  - The notice is conditional on data, not an always-present region, so no skeleton changes.

## Source + Goal linkage

- **Source:** [`w1/119`](../../done/119.md), filed by `w1/m150` t004 on 2026-10-02 and promoted 2026-10-02 by `/loopx w1`. It completes m150 § Decisions 2 (`../../done/m150/README.md`).
- **Goal linkage:** this is tenant-facing networking truth on the Render-compatible API (ADR006, ADR018's inbound-IP-allowlist row). The Networking card promises "Restrict inbound HTTP traffic to these source CIDRs", and that promise is silently false on orange-cloud hosts.
- **Expected outcome:** a tenant who restricts a service or environment whose custom domain is proxied by Cloudflare learns, on the surface they used, which hosts the allowlist cannot filter and how to fix it (switch the record to DNS-only). No existing configuration is refused.
- **Why now:** m150 made the allowlist match the real TCP source. That fixed DNS-only hosts and made the Cloudflare case the only remaining way the card can be wrong. The warning was promised in m150 and never built.
- **Render parity:** included. This adds a field to the REST, GraphQL and MCP responses and a dashboard notice. It is a bex extension with no Render counterpart, recorded on ADR018's inbound-IP-allowlist row.
