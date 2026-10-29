# w8 · m108 — Preserve ordinary uploads beyond the ingress body-read deadline

**Worker:** worker8 **Goal:** ordinary progress-making uploads finish within the supported web-service request budget. **Status:** superseded（2026-10-29：已被 w8/207 取代，见文末记录）

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

**More live evidence (w8 `/qa-find-bugs-cli` sweep 7980, 2026-10-26 21:10–21:56 UTC, pin `b4726243e74f`; the same 60 s deadline is also tracked as [w8/207](../../done/207.md), whose fix is live (2026-10-28)):**

- A native Python body-reading fixture received a fast 1.33 MB POST in full (`got=1333337 of 1333337 in 10.5s`). The same body at 10 KB/s got `502 Bad Gateway` after 61.67 s, and the app logged `got=620377 of 1333337 in 60.0s`, cut at exactly 60 s.
- Responses are not limited: `traefik/whoami ?wait=` returned 200 at 65, 125, 180, 330 and 420 s, so only the request-read phase is bounded (Traefik `readTimeout`; `writeTimeout` 0).
- `*.onbex.co` resolves to the Hetzner LB (`49.12.20.236`), not Cloudflare, so Cloudflare's 100 s limit is not involved.
- Render's first-party docs export (`render.com/docs/llms-full.txt`, 2026-10-26) states no request-duration or upload limit; "100 minutes" appears only in community reports.
- The tenant edge has no per-IP connection or in-flight limits today (`rg -i 'inFlightConn|inFlightReq|rateLimit' deploy/gitops` finds none). That is relevant to the slow-client side of this decision.

## 解除阻塞记录（2026-10-29，`/unblock w8`）——已被取代

- **涉及 ID：** w8/m108
- **结论来源：** 本次复核证据（main `843d08be0`，生产 pin `b33dd43c4428`）。
- **复核结果：** 本条目待定的「入口读预算」已由 [w8/207](../../done/207.md) 解决：`838feec85 fix(edge): allow bounded long request uploads` 把 `web`/`websecure` 的 `transport.respondingTimeouts.readTimeout` 设为 `6000s`（Render 的 100 分钟），即本设计的选项 1。[ADR022 § Bounded request reading at the tenant edge](../../../../docs/ADR022-tenant-isolation.md) 记录了 slow-client 风险的取舍，`scripts/gitops-validate.sh` 防回退，207 已按线上验收关闭（`bab5e727c`）。
- **处置：** 作为 superseded 归档到 `done/`。这不表示本条目自己的任务或线上验收通过；剩余的 header 超时拆分（选项 3，上游 Traefik）不再是 parity 所需，如需要另立新项。
- **用户门槛：** 无。
