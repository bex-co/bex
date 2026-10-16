# w8 · m83 — Supply automatic compression for web responses

**Worker:** worker8 **Goal:** eligible ordinary web responses transfer compressed with exact decoded content and correct neighboring HTTP behavior. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Wire compatible automatic web compression | 50m | — |
| [t002](t002.md) | Audit shared route callers and HTTP neighbors | 25m | t001 |
| [t003](t003.md) | Render parity | 20m | t002 |
| [t004](t004.md) | Simplify | 10m | t003 |
| [t005](t005.md) | Test coverage | 35m | t003 |
| [t006](t006.md) | Closeout | 20m | t004, t005 |

Six tasks,160m total.

## Definition of done

- Fresh permitted Free web services through both native clients reach first-deployment Live with exact source/artifacts, then eligible responses negotiate smaller Brotli/gzip over HTTP1/2 and decode exactly without tenant changes.
- Current identity, authored gzip, no-transform and separate finite SSE events keep working; meaningful negotiation/framing/validator/range and other shared routing neighbors are verified on the actual reviewed proxy.
- Maintained projection and actual-proxy tests fail before/pass after; shared routing/ownership/cost bounds and existing public-surface settings are audited. Required checks and explicit live/source limits are recorded.
- Owned services/children/metadata are absent, original baselines remain and quotas terminate at0. Tasks/milestone move to done through /pm with synchronized statuses and no stubs; static m79 retains its independently accepted scope.

## Source + Goal linkage

- **Source:** [functional CLI sweep352](evidence/cli-sweep352/README.md), [edge diagnosis](finding.md).
- **Goal linkage:** ADR008 usable hosting, ADR018 web-service compatibility and the current shared ingress architecture.
- **Expected outcome:** compressible web responses transfer fewer bytes while exact content and streaming behavior are preserved.
- **Why now:** both native services are healthy, all16 payload checks pass, but10 normal encoding offers stay at full size; authored gzip proves the transfer saving and source projection lacks a compressor. No matching repair is on fresh main.
- **Render parity included:** Render documents load-balancer Brotli/gzip for app responses. Identity is legal HTTP; this is a minor hosting/performance gap, distinct from completed static-handler m79.
