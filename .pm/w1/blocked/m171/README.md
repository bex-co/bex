# w1 · m171 — Warn when an inbound IP allowlist sits behind a Cloudflare-proxied custom domain

**Worker:** worker1 **Goal:** a tenant who saves an inbound IP allowlist on a service, or on an environment, whose custom domain is proxied by Cloudflare is told on every surface that the allowlist sees Cloudflare's address there, not the client's. The save still succeeds. **Status:** blocked on deploy + live verify. t001–t007 are done; only t008 (Closeout) remains, and it needs the change running in production and probed there.

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
| t008 | Closeout (deploy + live verify) | 30m | t006, t007 |

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
