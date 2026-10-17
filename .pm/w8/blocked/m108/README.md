# w8 · m108 — Preserve ordinary uploads beyond the ingress body-read deadline

**Worker:** worker8 **Goal:** ordinary progress-making uploads finish within the supported web-service request budget. **Status:** blocked — needs the user's choice of ingress body-read budget ([decision](design.md)); no task started

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Configure the public ingress body-read budget](t001.md) | 45m | — |
| t002 | [Audit shared entrypoint consumers and transfer forms](t002.md) | 45m | t001 |
| t003 | [Verify hosting duration parity across authoring surfaces](t003.md) | 25m | t002 |
| t004 | [Simplify ingress timeout changes](t004.md) | 15m | t003 |
| t005 | [Test actual chart arguments and streamed body completion](t005.md) | 60m | t003 |
| t006 | [Accept original paced uploads and close milestone](t006.md) | 20m | t004, t005 |

**Total:** 6 tasks / 210m. [Researched finding](finding.md) · [complete evidence](evidence/cli-sweep441/README.md).

## Definition of done

The pinned base/prod chart renders an explicit reviewed finite body-read budget that supports ordinary web-service uploads beyond60s and matches the documented product duration policy. Actual pinned-proxy regressions verify complete bytes/hashes with identical bodies/apps/pacing, fast controls, below/above-budget behavior and applicable transfer forms, while preserving shared HTTP consumers and TCP routes. After rollout both unchanged native clients' original fresh Free services accept the paced77s one-MiB body with full count/hash and native complete200 app/request evidence. Source/config readbacks, exact cleanup, baseline/current quota checks and relevant validation pass. Document actual protocol/runtime/duration/topology limits; diagnostic overrides, code or rendered strings alone do not meet live acceptance.

## Source + Goal linkage

- **Source:** Authorized functional CLI sweep441: both unchanged creates201/native0; both fast one-MiB uploads200; both paced uploads499 at60.6s with matching819,200-byte prefixes, app aborts and native edge logs. A direct77s control succeeds; changing only the local pinned proxy read timeout from default to100s also succeeds.
- **Goal:** [ADR008 hosting reliability](../../../../docs/ADR008-vision.md), [ADR018 Render parity](../../../../docs/ADR018-render-parity.md), [ADR004 deployment/request path](../../../../docs/ADR004-app-deployment.md): ordinary progress-making uploads complete within the supported request duration.
- **Expected outcome:** Users can upload a small file over a slower connection without the platform truncating its body at60s.
- **Why now:** The prior396 concern was inconclusive. Controlled pacing, actual consumed-prefix hashes, repeated first-service results and a one-setting local correction now isolate the incoming-body deadline with concrete regression evidence.
- **Render parity:** Included because shared ingress policy changes tenant-visible hosting behavior across API-authored and dashboard/Blueprint-authored services. Preserve existing surface fields/commands and explicit backend limits; no CLI fork or new selector.

## HTTP/2 upload follow-up — sweep442

[Complete HTTP/2 follow-up evidence](evidence/cli-sweep442/README.md) confirms the same60s incoming-body deadline on two fresh native-created Free Node24 services. Both fast one-MiB PUTs200/full hash; both paced PUTs502 after60.606/60.570s,819200 submitted and consumed bytes. Native app aborts, independently verified prefix hashes and HTTP/2.0 edge502 records agree, while original boots stay healthy. The original HTTP/1 response was499. Direct local TLS/HTTP2 succeeds at77.620s. Pinned local Traefik3.7.5 reproduces502 at60.024s; changing only readTimeout to diagnostic100s allows the full paced200/hash. Exact local/hosted cleanup is verified. Preserve these protocol-specific statuses in regression evidence; do not require every failed protocol to return499. The milestone remains todo and its live post-fix acceptance is outstanding.
