# ADR110: Security review round 25 — the operator's tenant-facing daemons and the dashboard's server routes

- **Status**: Accepted (2026-11-14)
- **Review**: the repository's `security-audit` skill, run 1 (`~/security-audit-skill/bex-w21/run-1`, outside the repository), at repository revision `cc671cbfd`. Nine hunters, seven adversarial validators and ten independent verifiers, each finding reproduced on the real binary, handler, bundle or Postgres where it could be. Ten findings confirmed (one Medium, nine Low); nine of the ten needed line or wording corrections, none was fabricated, and two remediation designs were corrected.
- **Lineage**: twenty-fifth pass in the ADR028 → … → ADR085 lineage (with the focused ADR103 and ADR107). ADR107's coverage note asked this round to weight the operator and the dashboard's server-side routes.
- **Work item**: `.pm/w21/m394`

## Summary

No exploit in this round crosses a tenant boundary to read another tenant's data, spoof a source address, escape a sandbox or reach a platform credential. Allowlists, PROXY-protocol and ClientHello parsing, SNI routing, static-server key construction, snapshot restore, the Hydra consent and device gates, and Key Value's internal proxy held under fuzzing, harnesses and real-binary tests. The defects are last-mile bounds: a missing cap, timeout or accounting term. All ten findings and three hardening items are fixed, each with a regression test that fails on the old code. One residual needs an ADR amendment (R25-12, below), and one hardening item is filed (R25-14).

| # | Severity | Finding | Disposition |
| --- | --- | --- | --- |
| R25-08 | Medium | An unmatched `/assets/**` path served the visitor's server-rendered page, with their Kratos session JSON (email, name, identity and session id, device IP, user agent, location), as `public, max-age=31536000, immutable`, so a shared cache could keep one user's page under an attacker-chosen URL | Fixed (t004, `9484c8339`): a missing asset answers 404 `no-store`; no `text/html` response may be `public`. The Cloudflare half needs deployment testing (§ Operator actions) |
| R25-05 | Low | Four attacker-chosen sites each held the static server's 4 origin-fetch slots, so every other tenant's cold objects answered 503 on that replica | Fixed (t009, `b87b7a518`): per-client in-flight cap; negative cache entries have their own budget and never displace real objects |
| R25-03 | Low | The operator followed a registry Bearer realm such as `http://127.0.0.1@host/…`, `127.0.0.1.attacker.example` or `127.0.0.100` and sent a plaintext GET to any in-cluster host | Fixed (t010, `70e8c1650`): the realm is parsed; https, or http on an exact loopback host |
| R25-01 | Low | The activator kept routing a host the owner dropped from a hibernated or suspended App (its `Status.URLs` was never refreshed), so a later claimant's requests could be woken, held and proxied to the former holder's app | Fixed (t011, `3c62f4b0e`): a parked App's status follows its spec; spec claims beat status-only ones; duplicate claims resolve deterministically |
| R25-02 | Low | The activator held a global and per-App slot through the whole proxied exchange, with no response-header timeout: four hibernated apps that accept and never answer pinned 256 slots | Fixed (t012, `0348c50c6`): the slot covers only the wake wait; a silent app ends at a header timeout (100 minutes, Render's limit; corrected from 100 s by w8/329, `b2b4bee9d`) |
| R25-10 | Low | The 48-hour reclaim bound on an unverified custom-domain claim restarted on delete and re-add; Blueprint sync and service create never reclaimed an expired claim | Fixed (t013, `3b370367d`): migration `0260` records a deleted pending claim's first-claim time and a re-add inside a day inherits it; `ReplaceDomainClaims` and `DomainHostsClaimed` honour the bound. ADR005 amended |
| R25-04 | Low | A stalling tenant registry held an App reconcile worker for the 5 s read budget per attempt; with 2 workers about 14 such Apps queued every other tenant's reconcile (a simulation, not a live measurement) | Fixed (t014, `831a60f40`): tenant-registry reads are capped per host and per workspace, a failing host backs off, an App's retries double |
| R25-06 | Low | The edge cache charged only body bytes against its budget: unique cacheable 404s reached 247 MiB RSS against a 160Mi limit at 250K requests | Fixed (t015, `e00af7b4c`): every entry is charged key, headers and a fixed overhead, expired entries are reaped; 84 MiB at 250K requests |
| R25-07 | Low | The edge-cache sidecar replaced `X-Forwarded-For` and `X-Forwarded-Proto` with the Traefik pod address and `http` | Fixed (t015): both are passed through, the peer appended |
| R25-09 | Low | `classifyHref` tested the raw string, so a raw-HTML link such as `<a href="&#9;//evil.example/x">` rendered as internal while the browser navigated off-origin | Fixed (t016, `330f06972`): the destination is classified as the browser reads it |
| R25-11 | Hardening | The activator's custom maintenance-page fetch queued behind 16 slow tenant URLs, delaying other tenants' pages about 4 s | Fixed (t012): the acquire never waits; a stale snapshot, else the default page |
| R25-13 | Hardening | `pg-sni-proxy` read the backend's SSLRequest reply with no deadline | Fixed (t017, `86a844f33`) |
| R25-15 | Bug | `?lang=%25`, `?lang=%` and `?locale=%25zz` threw `URIError` and answered 500 | Fixed (t017) |
| R25-12 | Residual | The SNI proxies' global cap is taken before the PROXY header and SNI, so a few sources hold every slot for the 10 s handshake deadline | Accepted per ADR066, whose premise is amended below; the IPv6 /64 mitigation is fixed (t017); the rest is filed (w21/070) |
| R25-14 | Hardening | The shared disk-snapshot age private key and the full-bucket S3 pair are readable by the bex-api Kubernetes token | Filed (w21/069) |

Rejected claims: R25-01 "Attack B" for custom hosts (a host with no Ingress gets Traefik's 404 before the activator; `config/activator/platform-host-fallback-ingress.yaml` matches `*.onbex.co` only); R25-12's silent single-source variant (behind the load balancer with the PROXY protocol the header arrives first) and its CPU variant (a legitimate client was still served 74 of 74).

## Scope and verdicts

Every `lego/operator/cmd/*` binary:

| Binary | Reach | Verdict |
| --- | --- | --- |
| `activator` | the public internet through Traefik; `bex-system`, `traefik` and `monitoring` pods | R25-01, R25-02, R25-11 fixed. The ServiceAccount has no admission policy (w21/068) |
| `pg-sni-proxy` | the internet through the load balancer (PROXY protocol); in-cluster pods | R25-13 fixed; R25-12 residual (w21/070). Parsing held under 4.2M fuzz executions: no panic, no untrusted peer ever asserted a source |
| `kv-sni-proxy` | as `pg-sni-proxy` | R25-12 residual. No unbounded backend read (the SSLRequest window is Postgres's) |
| `kv-internal-proxy` | same-namespace pods and same-isolation-environment peers | No finding. Switching identity, reading the injected password and escalating ACLs all failed against real Redis 8.10 |
| `staticserver` | the public internet through Traefik | R25-05 fixed. Key construction held: 40 hand vectors and a 4,000-case token fuzz found no key outside a site's prefix, and 100K accepted redirects never left the site |
| `edge-cache` | Traefik, same-workspace pods, `bex-system` peers | R25-06, R25-07 fixed. Slow-leader blocking and the missing server timeouts are hardening (w21/073). Cache deception inside one tenant is Render's behaviour (ADR018) and accepted |
| `egress-meter` | in-cluster; node firewall opens only 22 and ICMP; tenant pods' host egress is denied | No finding |
| `health-probe` | inside the tenant's own container | No finding |
| `drain-guard` | in-cluster | No finding in the binary; its admission policy has gaps (w21/068) |
| `native-init` | PID 1 in the tenant's image | No finding |
| `native-resolve` | build-pod init container | No finding: 12,080 resolutions with 587 payloads put no marker into a Dockerfile line |
| `disk-snapshot` | tenant PVC contents and symlinks | No finding: 20 hostile tar streams were all confined by `os.Root`. R25-14 and the notes in w21/073 are hardening |
| `backup-encrypt` | opaque RDB from the backup CronJob | No finding; R25-14 concerns the shared Secret |
| `registry-migrate` | an operator's kubeconfig, an admin tool | No finding; credentials in argv (w21/073) |
| `manager` | `:8081` health, `:8080` metrics, `:9443` webhooks | R25-03, R25-04 fixed. Unauthenticated plain-HTTP metrics and development-mode logging are hardening (w21/073) |

The dashboard's server surface (no `createServerFn` exists):

| Route or file | Caller | Verdict |
| --- | --- | --- |
| `/auth/consent` (`hydra-consent.ts`) | anonymous; a session for non-trusted clients | No finding. PKCE, audience-implies-scope, verified-email and ops gates fail closed on every upstream error. Hardening in w21/072 |
| `/auth/device/` (`hydra-device.ts`) | anonymous; POST needs a session | No finding. Hardening in w21/072 |
| `/api/sessions` | session cookie | No finding |
| `/api/connected-agents` | session cookie | No finding |
| `/healthz` | anonymous | No finding |
| `server/security-headers.ts` and the Nitro hooks | every request | R25-08 fixed |
| `src/common/lib/search-params/server.ts`, `external-url.ts` | every request; agent and member markdown | R25-15, R25-09 fixed |

Policy note, not a vulnerability: `hydra-consent.ts` and `hydra-device.ts` implement a human consent screen and a device bridge, beyond `.pm/DO_NOT_DO.md`'s "at most a headless consent acceptor". That is the owner's decision; nothing here changes it.

## Decisions

- **A missing asset is a 404 that no cache keeps, in two layers.** A middleware answers `/assets/**` (later also `/_assets/**`, w4/m348 t003) that reaches it with 404 `no-store`; a response hook rewrites any `text/html` carrying `public` to `private, no-store`. The middleware is registered as a global one: nitro-nightly `3.0.1-20260310` passes a routed middleware's `{ route, handler }` record to h3 as the function and every matching request answers 500.
- **A fetch gate is per client as well as per site.** One visitor holds at most 3 origin fetches on a replica, so 13 of the 16 slots, and one of any site's 4, stay with others. The client is Traefik's `True-Client-IP`, else the peer, with IPv6 collapsed to its /64 (`netutil.SourceKey`, shared with the SNI limiter). A request with no usable address is not limited. Negative cache entries (confirmed misses and the incompressible-variant marker) have a per-site budget of about 3,000 entries and only ever evict each other.
- **Tenant-registry reads do not use a global slot.** A slot shared by every host and tenant would move the starvation onto other tenants' image deploys. Reads are capped at one per host and one per workspace, a failing host's breaker closes it for 30 s doubling to 5 minutes, and a turned-away read frees the worker at once. The platform's own registry is not gated.
- **A hold slot covers the wait for the wake, not the exchange.** A silent app, a long upgrade or a stream holds none. The exchange is bounded at Render's 100 minutes. This corrects the first value, 100 seconds, which came from the finding's "about 100 s" without being checked against Render (w8/329).
- **The 48-hour bound counts from a host's first unverified claim.** It is kept in triggers on `domains` (migration `0260`), so every add path honours it, including the cascade of a service delete. No TXT or other ownership proof is added: ADR005 forbids it.
- **The edge cache's byte budget is its memory bound, and it forwards the client.** An entry costs its body, key, headers and 1 KiB of structure, so unique 404s cannot outgrow the sidecar's limit; the app behind it sees the client address and scheme in `X-Forwarded-*`, as ADR004 says.

## Render parity

Render parity is the top product principle (ADR008), so each fix that changes what a tenant or visitor observes was compared with render.com. No fix creates a divergence, and no ADR018 sign-off was needed.

| Fix | Render | bex now |
| --- | --- | --- |
| t013: custom-domain claims | Render's custom-domains page documents no "already in use" text, no reservation period and no rule for an unverified domain ("The docs don't say", read 2026-11-14), so the 48-hour reclaim is bex's choice where Render is silent, already in ADR005 before this round. REST, GraphQL, MCP, Blueprint and the dashboard reach one `Service.AddDomain` and the same `ReplaceDomainClaims`, which map a refusal to one conflict, so the surfaces cannot disagree | matches: the refusal paths and error shapes are unchanged; only when a stale claim stops conflicting moved |
| t015: forwarded headers | Render's proxy appends the proxy address to the client's `X-Forwarded-For`, and the app also sees `CF-Connecting-IP` and `True-Client-IP` (Render's DDoS article and community threads, read 2026-11-14; no Render docs page names these headers). An app on Render sees `x-forwarded-proto: https` (ADR018, w8/m118) | matches: `X-Forwarded-For: client, peer`, `X-Forwarded-Proto: https`, `True-Client-IP` kept |
| t011: a dropped host | Render's custom-domains page does not describe a removed custom domain's routing ("The docs don't describe", read 2026-11-14), so there is no documented behaviour to compare | nothing to diverge from: a host the owner dropped is no longer routed to the service |
| t012: a woken request | Render allows HTTP responses up to 100 minutes (render.com/docs/render-vs-vercel-comparison) | matches after w8/329 |
| t009: overload | Render's static-sites page says the site is served over a global CDN and documents no per-visitor limit and no 503 | drift risk, not a regression of one commit: a visitor's 4th concurrent cold asset now sheds with 503 where the 5th did; filed as w21/067 |
| t004, t016, t010, t014, t017 | no Render counterpart (dashboard internals, the operator's registry client and queue, proxy limits) | not applicable |

## Residuals

- **Accepted, unchanged:** `.pm/DO_NOT_DO.md` #PSL, #CI-RUNNERS, #RUNNER-HOSTS, #ENV-REVIEWERS, #APPROVAL-MODE.
- **Accepted per this ADR:**
  - R25-12, the global-cap residual. ADR066 accepts source-rotating floods as "edge/LB rate limiting"; its premise is false: the Hetzner load balancer has no L4 limiter and nothing in the repository provides one, and no alert watches `connections_rejected_total`. The control lives in the pod; ADR066 is amended and the work is w21/070.
  - A squatted pending claim that later verifies (the shared-edge apex proof). Mechanically real, and Render's own apex structure; ADR005 records it.
  - Cache deception inside one tenant (a cookie-authenticated `/account/x.css` cached for anonymous clients on an opted-in service): Render's edge-cache behaviour (ADR018), per pod, no cross-tenant effect.
- **Filed as hardening:** w21/067 (static cold-start shedding, Render drift risk), w21/068 (admission policies), w21/069 (R25-14, snapshot keys), w21/070 (SNI proxy pre-SNI cap, pre-header deadline, alert), w21/071 (`pods/exec`, six namespaces without a NetworkPolicy), w21/072 (dashboard notes), w21/073 (operator proxy, job and logging notes), w21/074 (the backend's per-source caps in the SSH gateway and auth admission key an IPv6 client on its exact address; `netutil.SourceKey` is the fix).
- **What a single run leaves:** the control plane in `lego/backend` was ADR107's scope. The first run on these planes covered every binary and route, unevenly: deepest on the activator, SNI proxies, static server, edge cache, registry client, snapshot jobs and the consent and device flows.

## Operator actions this round leaves

These are not repository work, and the first two are what t004 could not do (the deploy token is DNS-only):

- **Cloudflare, dashboard zone:** a cache rule that bypasses the cache for requests carrying the Kratos session cookie, and a one-time purge of `/assets/*` (and `/_assets/*`), so a page cached under an attacker-chosen URL before the fix cannot outlive the fix.
- **Needs deployment testing (not confirmed):** whether Cloudflare stored the personalised `/assets/**` page; whether the Hetzner load balancer sends the PROXY header at connect; Hydra open dynamic client registration without `disallow_private_ip_ranges`; the activator's maintenance fetch to its own node's public IP (ADR103 #10's "no Hetzner firewall" premise looks stale); Kratos rejecting `return_to=https://evil.com/`; BuildKit with a symlinked context root; Wasabi's handling of `..` in S3 keys; Traefik's `Host()` canonicalisation and its handling of untrusted `X-Forwarded-*`; a live probe from a tenant pod to `pg-sni-proxy:5432`; the Argo CD controller token through `pods/exec`; `openstatus.onbex.co` CSRF for same-site requests. A production check is owed after the activator rolls: a hibernated free service answers `?wait=130s` with 200, not 502 (w8/329).
- **Coverage:** a second run should weight the manager's controllers (`internal/controller` is about 30K lines: finalizers, per-namespace Secret writes, Ingress and middleware generation from tenant input) and the sandbox and agent pod paths, which this round covered least, then `native-init`, `health-probe` and the dashboard's client-only code beyond markdown.

## Verification

Regression tests fail on the pre-fix code and pass after it:

- dashboard `assets-cache-guard.test.ts` (the miss handler and the response hook), `external-url.test.ts` and `markdown-renderer.test.tsx` (raw-HTML links), `search-params/__tests__/server.test.ts`
- `staticserver.TestOneClientCannotStarveOtherTenantsSites`, `TestOneClientCannotPinAWholeSite`, `TestClientKey`, `TestNegativeEntriesCannotEvictRealObjects` and its global variant
- `registry.TestBearerRealmIsHTTPSOrExactLoopback`, `TestBearerServiceAndScopeAreEscaped`
- `controller.TestParkedAppStatusURLsFollowTheSpec`, `TestAutoHibernatedAppStatusURLsFollowTheSpec`; activator `TestHostIndexSpecClaimBeatsStaleStatus`, `TestHostIndexDuplicateSpecClaimIsDeterministic`
- activator `TestHungAppDoesNotPinHoldSlots`, `TestUnansweringAppEndsAtTheResponseHeaderTimeout`, `TestUpgradeThroughTheHoldPathStillWorks`, `TestBusyMaintenanceFetchServesTheStaleSnapshotAtOnce`
- `store.TestAddDomainClaimDeleteReaddKeepsExpiry`, `TestReplaceDomainClaimsReclaimsExpiredPendingSquat`, `TestDomainHostsClaimedIgnoresExpiredPendingClaims`, `TestClaimHistoryIsBounded`, against real Postgres
- `controller.TestTarpitRegistryCannotHoldEveryWorker`, `TestOneWorkspaceCannotReadSeveralTarpitsAtOnce`, `TestImageRegistryWaitBacksOffPerApp`; `registryReadGate` unit tests
- `edgecache.TestUniqueNotFoundsStayWithinTheMemoryBound`, `TestAnEntryIsChargedItsKeyAndHeaders`, `TestExpiredEntriesAreReapedWhenAnotherIsStored`, `TestForwardedHeadersSurviveTheCache`; the real binary under the production settings, 250,000 unique 404s: 247 MiB before, 84 MiB after
- `pg-sni-proxy.TestStalledBackendSSLReplyReleasesTheSlots`; `sniproxy.TestLimiterPerSourceKeysIPv6OnSlash64`; `netutil.TestSourceKey`

The real-database backend suite (`scripts/ship-gates.sh`), `make test` and `make lint` for the operator, and the dashboard's `yarn lint` and `yarn test` are the release gates. The skill's `findings.json` validates against its schema.
