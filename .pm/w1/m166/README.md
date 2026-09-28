# w1 · m166 — Truthful Key Value readiness

**Worker:** worker1 **Goal:** Available means the Key Value client path is serving, not merely that a plaintext socket accepts connections. **Status:** todo (t001 done — the gap is measured at ~25s and is a *flap*, not a gap)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Measure the Key Value readiness gap — **DONE** | 35m | — |
| t002 | Probe authenticated serving readiness | 45m | t001 |
| t003 | Publish Ready only for the serving generation and route | 40m | t002 |
| t004 | Render parity | 25m | t003 |
| t005 | Simplify | 20m | t004 |
| t006 | Test coverage | 35m | t004, t005 |
| t007 | Closeout | 10m | t005, t006 |

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

## Source + Goal linkage

- **Source:** approved w1 brainstorm item 1, 2026-09-28; absorbs former w4/m137/t009, whose full evidence is [preserved here](source-w4-m137-t009.md). Completed work and the remaining production closeout remain in [w4/m137](../../w4/blocked/m137/README.md). No implementation is marked done by this transfer.
- **Goal linkage:** ADR008 pillar 2 agent-readable state; ADR021 managed Key Value and ADR018 Key Value row.
- **Expected outcome:** agents can safely proceed when a store reports available.
- **Why now:** live re-probe measured status leading successful client access by 10–23 seconds even after the status-label fixes shipped.
- **Render parity:** included; tenant-facing status crosses all four surfaces. User direction (2026-09-28): where behavior is uncertain, verify the Render contract and keep parity; do not introduce an undocumented divergence. Existing explicit non-goals remain out of scope.
- **Sizing:** 120m implementation; 210m total, seven tasks.
- **Environment:** use isolated dev-1; local harness recovery is pre-approved, but actual capacity failures must be reported honestly. No production rollout is authorized by filing.
