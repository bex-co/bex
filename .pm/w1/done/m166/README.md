# w1 · m166 — Truthful Key Value readiness

**Worker:** worker1 **Goal:** Available means the Key Value client path is serving, not merely that a plaintext socket accepts connections. **Status:** done 2026-10-02. t001–t007 are done. On the deployed fix (`2e35b42a1`), the production timeline shows no available-but-unreachable window beyond one sampling interval: gaps of 1.7s, 0.2s and 0.5s, against 24.6–26.5s before. See [t007-prod-verification-timeline.md](t007-prod-verification-timeline.md).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Measure the Key Value readiness gap — **DONE** | 35m | — |
| t002 | Probe authenticated serving readiness — **DONE** | 45m | t001 |
| t003 | Publish Ready only for the serving generation and route — **DONE** | 40m | t002 |
| t004 | Render parity — **DONE** | 25m | t003 |
| t005 | Simplify — **DONE** | 20m | t004 |
| t006 | Test coverage — **DONE** | 35m | t004, t005 |
| t007 | Closeout — **DONE** | 10m | t005, t006 |

## Definition of done

- Maxmemory changes, persistence changes and resume have dated concurrent external TLS PING and API-status timelines; there is no available-but-unreachable window beyond one sampling interval.
- A loading/authentication/TLS failure cannot satisfy readiness. Use authenticated checks without logging secrets.
- New resources say creating; previously served stores say config_restart while unavailable; suspension and generation guards remain correct.
- REST, GraphQL, MCP and dashboard agree. Relevant operator/backend tests pass, with meaningful failing-before regressions.
- Root cause is measured, not assumed; local evidence and production evidence are distinguished explicitly.

## t001 measurement (2026-09-28, production, `/loopx w1`)

**Fixture.** `qa-20260928-m166` (`red-dat25gq1pbgc73a24llg`), a `starter` **public** Key
Value store, so an external `rediss://` endpoint exists to probe. Deleted afterwards
(`204`, then `GET` → `404`).

**Method.** Two clocks, one second apart, running concurrently for the whole window —
an external TLS connect + `AUTH` + `PING` written on stdlib `ssl` (so the timing is ours,
not a client library's retry policy, and the failure *kind* is recorded), and
`GET /v1/key-value/{id}.status`. Then the three transitions the DoD names. Full
interleaved timeline in `t001-timeline.md`.

**Result: the gap is real, it is larger than filed, and it is not a gap — it is a flap.**

| transition | API says `available` | client *stably* serving | window |
| --- | --- | --- | --- |
| maxmemory → `allkeys_lfu` | +28.6s | **+53.2s** | **24.6s** |
| persistence → `snapshot` | +140.5s | **+166.8s** | **26.3s** |
| resume | +336.3s | **+362.8s** | **26.5s** |

The note's "10–23 seconds" understated it, but the more important correction is the
**shape**. After the API reports `available` the client does not simply start working —
it alternates, three or four times, over roughly 25 seconds. From the maxmemory case:

```
 28.6s  [api   ] status=available
 32.3s  [client] SERVING          <-- a single probe here would have passed
 33.6s  [client] DOWN(tls:SSLEOFError)
 36.8s  [client] SERVING
 38.1s  [client] DOWN(tls:SSLEOFError)
 41.2s  [client] SERVING
 46.1s  [client] DOWN(tls:SSLEOFError)
 53.2s  [client] SERVING          <-- stable from here
```

**This changes what t002/t003 can be.** "Probe once before publishing Ready" — the
obvious reading of t002 — is **not sufficient**: a probe at +32.3s succeeds and Ready
would publish into a path that fails again 1.3s later. Readiness has to require
*sustained* success (N consecutive probes spanning the flap, or a settle window), or the
flap itself has to be removed. Whichever is chosen must be recorded in § Decisions.

**The failure kind points at the cause.** The dominant failure is
`tls:SSLEOFError` — the TLS handshake reaches something that closes the connection,
rather than `conn:refused` (nothing listening) or `-LOADING` (Valkey up but not ready).
That is the signature of the connection being **routed to an endpoint that is not
serving** — a terminating pod still in the Service's endpoint set, or the kv-sni-proxy
still holding the old backend — not of Valkey being slow to load. So the readiness bug
is likely a *routing/endpoint* truth problem, which is also what makes it flap: each
probe is a coin toss over a stale and a live endpoint.

**Controls that behaved correctly**, and should stay that way:

- `suspend` → API `suspended` at +250.1s and the client is DOWN throughout — no
  available-but-unreachable claim while suspended.
- A newly created store reported `creating` for ~48s before `available` (the earlier
  create in this session), so the DoD's "new resources say creating" already holds.
- `config_restart` is reported for a previously-served store while unavailable, on all
  three transitions — the other DoD bullet that already holds.

**Not yet measured (t002's remaining scope):** whether a `-LOADING` reply or an auth
failure can *itself* satisfy readiness, which needs an authenticated in-cluster probe
rather than this external one, and must not log secrets.

## t002 gate (2026-09-28, `/loopx w1`)

**t001's measurement redirected this milestone, and the redirection is what blocks
t002.** The milestone was framed as "probe authenticated serving readiness before
publishing Ready". t001 shows a single probe cannot work — it would have passed at
+32.3s into a path that failed 1.3s later. Two ways forward, and they are not
equivalent:

1. **Require sustained success** (N consecutive probes, or a settle window spanning the
   ~25s flap). Implementable from here, in the operator, with envtest coverage. But it
   *masks* the flap rather than fixing it, and this milestone's DoD says outright that
   "root cause is measured, not assumed".
2. **Fix the flap.** t001's evidence points at it: the dominant failure is
   `tls:SSLEOFError` — the connection reaches something that closes it — not
   `conn:refused` (nothing listening) or `-LOADING` (Valkey up, still loading). That is
   the signature of a stale endpoint still in the Service's set, or kv-sni-proxy holding
   the old backend, and it explains the coin-toss shape. If that is the cause, Ready is
   publishing truthfully about the *pod* while the *route* is wrong, and a readiness
   probe is the wrong layer to fix it at.

**Choosing (2) — the one the DoD points at — requires watching endpoints through a
transition, and this session cannot.** It needs `kubectl get endpoints` / the
kv-sni-proxy's backend set sampled across a maxmemory change, which means:

- **production cluster access** — not available here: there is no `infra/*.kubeconfig`,
  and `BEX_PROD_KUBECONFIG`/`KUBECONFIG` are absent from `.env`. `HCLOUD_TOKEN` is
  present, but minting cluster access from it is an outward-facing infrastructure action
  and not something to do unprompted; or
- **a healthy local cluster** — the same gate that parks `w1/m163` and `w1/108`. The
  local app cluster is reachable again after this session repaired its kubeconfig port
  drift, but its control plane is not stable: `kube-scheduler` has **350** restarts and
  `kube-controller-manager` **354**, each running ~6 minutes before exiting 1, for nine
  days. Stabilizing it means a `mock-cluster.sh` reprovision, which wipes every other
  workstream's `dev-N` stack while a concurrent session is active.

**To clear, pick one:** authorize the reprovision (and accept the `dev-N` blast radius),
supply a production kubeconfig for read-only endpoint observation, or decide that
option (1) is acceptable and this milestone may mask the flap — in which case say so and
t002/t003 proceed immediately, since nothing else gates them.

## Root cause (2026-10-02, **production**, read-only cluster + QA API)

**User decision (2026-10-02): root-cause the flap on production** — this clears the t002
gate above (option 2, fix the flap). Fixture `qa-20261002-m166`
(`red-davkbd38dvus73e89p30`, starter, public), deleted afterwards (`204`, `GET` → `404`).
Full five-sampler timeline: [t003-prod-rootcause-timeline.md](t003-prod-rootcause-timeline.md).

**Mechanism: the KeyValue's `<id>` Service was headless, and CoreDNS caches its answers
for 30s on each of two replicas.** kv-sni-proxy dials
`<id>.<ns>.svc.cluster.local:6380` per connection. For a headless Service that name's A
record *is* the pod IP, so a restart moves it pod IP → NXDOMAIN (no ready endpoint) → new
pod IP. The Corefile has `kubernetes … { ttl 30 }` + `cache 30` and there are two CoreDNS
replicas with independent caches, so for up to 30s after the new pod turned Ready each
lookup was a coin toss between a fresh answer and a stale one (old IP or NXDOMAIN). The
public client's connection lands on one of three proxies, each of which resolves through
either CoreDNS replica — hence the flap rather than a clean gap.

Key evidence lines (UTC, production):

```text
resume
05:51:42.943Z [endpts] ready=[10.244.34.140]                       <- new pod is the sole ready endpoint
05:51:43.576Z [api   ] status=available                            <- truthful about the pod (+0.6s)
05:51:43.193Z [proxy ] xtz4w dial backend: lookup <id>.<ns>.svc.cluster.local on 10.96.0.10:53: no such host
05:51:47.373Z [client] SERVING
05:51:49.182Z [client] DOWN(tls:EOF)    + proxy gpcn2 "no such host" at the same instant
   ... SERVING / DOWN(tls:EOF) alternate 6 more times, each DOWN matched 1:1 by a proxy NXDOMAIN ...
05:52:10.357Z [proxy ] w5xmh dial backend: ... no such host        <- last cached NXDOMAIN, 27.4s after Ready
05:52:11.405Z [client] SERVING                                      <- stable: 27.8s after available
maxmemory -> allkeys_lfu
05:49:46.815Z [endpts] ready=[]                                     <- old pod 10.244.34.249 removed at once
05:49:57Z..05:50:19Z [proxy] all 3 pods: dial tcp 10.244.34.249:6380: i/o timeout   <- cached OLD pod IP
05:50:09.951Z [pods  ] new pod 10.244.34.120 Ready ; 05:50:10.519Z [api] available
suspended (no pod at all)
05:50:51.138Z [proxy ] xtz4w ... no such host
05:50:51.956Z [proxy ] w5xmh dial tcp 10.244.34.120:6380: i/o timeout   <- the two caches disagree, 0.8s apart
```

- The client's `DOWN(tls:EOF)` is t001's `SSLEOFError`: the proxy accepts, reads the
  ClientHello, fails the backend lookup instantly and closes. `DOWN(timeout)` is the
  cached old IP (a 10s `dialTimeout` outlasts the 3s client timeout).
- The resume window (27.8s) matches t001's 26.5s. In this run's maxmemory case the new pod
  took 23s to get an IP and become Ready, so most of the 30s cache had already expired
  — the old-IP dials ended 05:50:19 and the client was stable at +3s. Same mechanism;
  how much of it is visible after `available` depends on how fast the pod restarts.

**Hypotheses ruled out, with evidence:**

| hypothesis | verdict |
| --- | --- |
| terminating pod still in the endpoint set (`publishNotReadyAddresses` / terminating endpoints) | **No.** Service has no `publishNotReadyAddresses`; the old IP left `ready=[]` at 05:49:46.8, the same second the pod got its `deletionTimestamp`. The proxies kept dialing it for 23s from *DNS*, not endpoints. |
| Deployment rollout with surge (old and new both serving) | **No.** It is a one-replica StatefulSet, `RollingUpdate`/`OrderedReady`: the old pod is gone before the new one is created (pods sampler never shows two). |
| kube-proxy/IPVS connection reuse | **No.** Headless Service — no VIP, no kube-proxy/Cilium service translation is involved at all; the proxy dials the pod IP DNS gave it. |
| Valkey TLS readiness semantics (6380 lagging 6379) | **No** for the flap: zero `connection refused` on 6380 to the *new* IP; every failure is a lookup or a dial to the *old* IP. (The TCP probe was still too weak — see t002 below.) |
| operator marks available before the route converges | **Partly, and the route was the DNS layer.** `available` follows the Endpoints update by <1s; the operator's pod/generation gates are correct. What lagged was CoreDNS, which no operator condition observed. |

**Postgres control explained:** CNPG's `-rw` Service (what pg-sni-proxy dials) is a
ClusterIP — its A record never changes — which is why t001-era Postgres restarts had no
flap.

## Decisions

- **2026-10-02 — Fix the flap at its cause: the `<id>` Service becomes ClusterIP**
  (`keyvalue_controller.go` `reconcileKeyValueService`). The A record is then the
  Service VIP for the Service's whole life, so neither CoreDNS's positive nor negative
  cache can be stale; the dataplane re-points the VIP within ~1s of the endpoint turning
  Ready (the same path Postgres already uses). This also fixes **in-cluster tenant Apps**,
  which resolve the same headless name and were exposed to the same 30s stale window.
  Rejected: (a) a sustained-success settle window (masks the flap, and DoD forbids guessing
  from elapsed time); (b) teaching kv-sni-proxy to watch EndpointSlices (new cluster-wide
  RBAC, and fixes only the public path); (c) lowering CoreDNS `cache`/`ttl` (cluster-wide
  blast radius, still a window).
- **Migration:** `spec.clusterIP` is immutable, so an existing headless Service the
  KeyValue controls is deleted (UID precondition) and recreated as ClusterIP on the next
  reconcile. **Deploy impact:** one brief internal-DNS blip per existing store when the
  new operator first reconciles it (the name is NXDOMAIN between delete and create, and
  that NXDOMAIN may be cached up to 30s); and the pod-template change (probe) rolls every
  KeyValue StatefulSet once. Both are one-time; schedule the rollout accordingly.
- **2026-10-02 — Readiness is an authenticated PING (t002).** The `valkey` container's
  readiness probe is `sh -c '[ "$(REDISCLI_AUTH="$VALKEY_PASSWORD" valkey-cli -p 6379 PING)" = PONG ]'`
  every 5s (timeout 2s): `-LOADING`, `NOAUTH`/`WRONGPASS` and refused all fail it. The
  password reaches valkey-cli through `REDISCLI_AUTH` (the backup snapshot step's
  mechanism), never argv. No separate TLS-port check: Valkey binds 6379 and 6380 together
  before loading, and production showed no 6380-specific failure.
- **No extra Ready gate (t003).** With a ClusterIP backend the route converges within one
  sampling interval of the pod's Ready, which the existing current-revision pod gate
  already observes; adding a route probe in the operator would be a fixed-sleep in
  disguise. Generation, `creating` vs `config_restart`, and suspension precedence are
  unchanged (backend `kvStatus` untouched).
- **Evidence labelling:** everything in § Root cause is **production** (2026-10-02, old
  build). The fix's verification so far is **unit + envtest only**; no production or local
  timeline on the fixed build has been run — that is t007.

## t004 Render parity (2026-10-02)

- **Observed (production, old build):** REST `status` moved `available → config_restart →
  available` on maxmemory and `suspended → config_restart → available` on resume — the
  Render `databaseStatus` vocabulary Key Value reuses. Nothing in this fix changes the
  vocabulary or the mapping.
- **Inferred from code:** GraphQL, MCP and the dashboard read the same `KeyValueView`
  built by `kvStatus` (`lego/backend/internal/keyvalue/service.go`), so they agree with
  REST by construction. Render documents no internal-host/DNS behavior that a ClusterIP
  Service would diverge from; the connection strings are byte-identical. ADR018's Key
  Value row records the change.

## t005 / t006 (2026-10-02)

- Simplify: the change is two small functions plus one probe constructor; no readiness
  abstraction or duplicate policy was added (`currentRevisionPodsReady` still the one gate).
- Regressions (failing before, passing after): `TestValkeyReadinessProbeIsAuthenticatedPing`
  and `TestValkeyReadinessProbeScript` (runs the projected probe command against a stand-in
  `valkey-cli`: PONG passes; LOADING / WRONGPASS / NOAUTH / refused fail) — verified to fail
  against the old TCP probe; envtest `reconciles a tier-sized StatefulSet + ClusterIP
  Service…` and `replaces a pre-w1/m166 headless Service with a ClusterIP one` (also pins
  that a headless Service the KeyValue does not control is left alone) — verified to fail
  against the old headless Service.

## Source + Goal linkage

- **Source:** approved w1 brainstorm item 1, 2026-09-28; absorbs former w4/m137/t009, whose full evidence is [preserved here](source-w4-m137-t009.md). Completed work and the remaining production closeout remain in [w4/m137](../../w4/blocked/m137/README.md). No implementation is marked done by this transfer.
- **Goal linkage:** ADR008 pillar 2 agent-readable state; ADR021 managed Key Value and ADR018 Key Value row.
- **Expected outcome:** agents can safely proceed when a store reports available.
- **Why now:** live re-probe measured status leading successful client access by 10–23 seconds even after the status-label fixes shipped.
- **Render parity:** included; tenant-facing status crosses all four surfaces. User direction (2026-09-28): where behavior is uncertain, verify the Render contract and keep parity; do not introduce an undocumented divergence. Existing explicit non-goals remain out of scope.
- **Sizing:** 120m implementation; 210m total, seven tasks.
- **Environment:** use isolated dev-1; local harness recovery is pre-approved, but actual capacity failures must be reported honestly. No production rollout is authorized by filing.
