# w6 · m566 — An agent session's sandbox is billed for the CPU it uses and the memory it is allocated, as a Render sandbox is

**Worker:** worker6 **Goal:** an agent session's sandbox pays what a `/v1/sandboxes` sandbox pays for the same running time, used CPU at $0.084 per CPU-hour plus allocated memory at $0.0133 per GiB-hour, instead of the weighted allocated-shape charge. The session's cost and spend ceiling use the same figures. **Status:** todo (t001 and t002 done 2026-11-24: on production every agent sandbox's used CPU and allocated memory are recorded as unbilled rows, and all 37 of the first day agree with the pod counter; t003 next)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Record agent sandboxes' used CPU, shown at $0 beside the weighted charge](done/t001.md) — **DONE** | 1h30m | — |
| t002 | [Check a day of agent CPU readings against the pod counter](done/t002.md) — **DONE** | 45m | t001 |
| t003 | [Cut agent sandboxes over to used CPU and allocated memory at one UTC hour](t003.md) | 2h | t002 |
| t004 | [A session's cost and its spend ceiling price the same CPU and memory](t004.md) | 2h | t003 |
| t005 | [ADR030 §17, ADR047 D6, ADR023 and the billing page say agent sessions are billed on use](t005.md) | 45m | t003, t004 |
| t006 | [Production check: an idle agent sandbox hour and a session's cost after the cutover](t006.md) | 1h | t005 |
| t007 | [Render parity](t007.md) | 30m | t006 |
| t008 | [Simplify](t008.md) | 20m | t007 |
| t009 | [Test coverage](t009.md) | 45m | t007 |
| t010 | [Closeout](t010.md) | 10m | t008, t009 |

## Definition of done

- From one UTC hour on, recorded as `sandboxUsage.agentBilledFrom` (or the name t003 chooses) in `lego/backend/internal/pricing/pricing.yaml`, an agent session's running sandbox is billed `sandbox_cpu_seconds` and `sandbox_memory_gb_seconds` at the `sandboxUsage` rates, in every workspace. Earlier usage stays on `sandbox_compute_seconds` and is not re-priced.
- On production after the cutover, an idle hour of a 4 CPU / 8 GiB agent sandbox previews about $0.106 of memory and almost no CPU, against $0.434 before.
- A session's `cost.computeUSD` and its `maxSpendUSD` ceiling count the same CPU and memory as the bill, on REST, GraphQL, MCP and the dashboard; a session's cost equals the sum of its sandboxes' rows to the micro-dollar.
- The billing page's note about billing on use names the agent-session date, and ADR030 §17, ADR047 D6 and ADR023 no longer say an agent session keeps the weighted meter.

## Source + Goal linkage

- **Source:** the recommendation the user approved on 2026-11-22 ("其他的按照你推荐的修"). Since 2026-11-18 a `/v1/sandboxes` sandbox is billed on use ([ADR030 §17](../../../../docs/ADR030-pricing.md)), but an agent session's keeps the weighted meter (`UsageBilled: idleTracked(raw)` in `lego/backend/internal/sandbox/meter.go`, and ADR030 §17's "Agent sessions" line). The billing page tells every reader that "a running sandbox is billed for the CPU it uses and the memory it is allocated", which is untrue for the largest sandbox line on it.
- **Evidence (production, 2026-11-22):** November's agent sandboxes in the `bex` workspace billed $231.64 on the weighted meter; their turns ran 21% of the billed hours.
- **Reference:** Render bills a running sandbox for actively used CPU and allocated memory ([ADR104 D8](../../../../docs/ADR104-sandbox-render-parity.md), render.com/blog/introducing-render-sandboxes, published 2026-11-04). Devin bills a session's awake time in ACUs and lets it sleep after 5 minutes (docs.devin.ai/admin/billing/usage). With m563 an agent sandbox is awake for little more than its turn, so billing on use and Devin's awake-time model come to nearly the same idle charge. The user approved billing on use.
- **Depends on:** w22/m314, the used-CPU meter for `/v1/sandboxes`. Production already carries it (`dcf1f1991`), and it bills since 2026-11-18. Its last accuracy run (`sandbox-measure.yml`, `leg=usage`) is still pending. t003's cutover waits until that run passes; t001 and t002 do not.
- **Goal linkage:** ADR023 usage metering, ADR030 pricing, ADR047 D6: one billing model for one product's sandboxes ([ADR008](../../../../docs/ADR008-vision.md)).
- **Expected outcome:** agent sandbox charges follow use. After m563 removes most idle time, the remaining running time is charged at memory plus the CPU actually used.
- **Why now:** the billing page's copy already promises it, and customers' agent sessions are the largest sandbox line.
- **Render parity:** included: the `cost` fields on REST, GraphQL and MCP, and the billing page.

## Unblock record 2026-11-24 (/loopx w6, UTC)

- **Source:** verified evidence, not a user decision. Affects t001, t002 and t003.
- **The gate cleared.** Production's pin `fa30302aef2a` (`9d6aed32e4`, 12:57Z) carries `506b9b40ef` (`git merge-base --is-ancestor 506b9b40ef fa30302aef2a`). The first pin to carry it was `9fdf7e0a1171` (`cded300699`, 2026-11-23T17:33Z), so by this read (18:17Z) production had metered agent sandboxes with t001's code for 24 h 44 min, and the date gate 2026-11-24T18:00Z had passed.
- **t003's other gate holds.** w22/m314's usage leg printed PASS in [sandbox-measure run 45147416800](https://github.com/bex-co/bex/actions/runs/45147416800) (2026-11-23T09:29Z), and w22/m314 closed on it. No usage-leg run has reached the meter checks since:
  - three re-runs that evening (45275218820, 45276212231, 45277163478) were cut by the exec-stream reset of w22/028 first;
  - the runs on 2026-11-24 were the `cpu` and `rate` legs.

  That PASS predates `506b9b40ef`. The part of the meter `506b9b40ef` changed for agent sandboxes is what t002 checks below, against the same counter.

- **t001's production check passed.** Read-only, on the replica `bex-db-1` of `hetzner-prod`.
  - 37 sandboxes not billed on use started a running interval after 17:00Z on 2026-11-23, every one interval: 18 `agent-4c-8g` sandboxes of `bex` workspace sessions, and 19 `2c-4g` sandboxes in `bex` and `tian-personal` that no session row adopts (neither `agent_session_sandboxes` nor `agent_sessions` names them). The first started 17:51:59Z.
  - Every interval was read by the CPU meter (`cpu_settled`).
  - Every sandbox has `sandbox_cpu_seconds` and `sandbox_memory_gb_seconds` rows marked `unbilled` beside its billed `sandbox_compute_seconds`.
  - The 18 session sandboxes: 20 hourly rows of each kind, 11,086 CPU-seconds and 85,986 GiB-seconds unbilled, 52,080,212 weighted milli-seconds billed.
  - Memory is allocated × running time, for example 8,751 GiB-seconds for 1,094 s of an 8 GiB sandbox.
  - No other row is marked.
- **t002 passed: all 37 agree with the pod counter.**
  - **Method.** For each sandbox, the raw samples of `sandbox_pod_cpu_usage_seconds_total{namespace="<workspace>-sandbox",pod="<pod_name>"}` over its running interval, read on production's Prometheus (`prometheus-server` in `monitoring`, `wget` inside the pod).
  - **Band.** w22/m314's band (`scripts/measure-sandbox-usage.sh`, "busy: metered CPU matches the pod-level counter"): recorded CPU-seconds, with the interval's `cpu_remainder_micros`, between (last − first sample in the interval) × 0.95 − 1 and the last sample × 1.02 + 2.
  - **Result.** 37 of 37 are inside it; 32 equal the counter's last sample to the hundredth. Recorded 11,339.42 CPU-seconds against the counter's 11,341.69 (0.9998). The session sandboxes alone: 11,094.51 against 11,096.02.
  - **The five below 1.0000.** Each pod was scraped before the meter saw its sandbox running, so the CPU it used before its interval opened is the baseline, not part of the reading: at most 0.54 CPU-second.
  - **The session sandboxes:**

| sandbox | started | running s | recorded CPU-s | counter, last sample | counter, first sample | band | recorded ÷ last |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `bf7127e4` | 2026-11-23T18:30Z | 1094 | 1884.31 | 1884.31 | 10.55 | 1779.1–1924.0 | 1.0000 |
| `b3f5ab98` | 2026-11-23T18:55Z | 523 | 348.18 | 348.18 | 29.79 | 301.5–357.1 | 1.0000 |
| `baaf5772` | 2026-11-23T20:01Z | 788 | 687.14 | 687.61 | 1.10 | 651.2–703.4 | 0.9993 |
| `2bdad44b` | 2026-11-23T20:15Z | 132 | 167.67 | 167.67 | 50.69 | 110.1–173.0 | 1.0000 |
| `8605c2eb` | 2026-11-23T22:25Z | 479 | 454.21 | 454.21 | 8.50 | 422.4–465.3 | 1.0000 |
| `648607ae` | 2026-11-24T00:00Z | 1192 | 1473.87 | 1473.87 | 6.19 | 1393.3–1505.4 | 1.0000 |
| `fc0d17f0` | 2026-11-24T00:21Z | 143 | 200.04 | 200.58 | 10.64 | 179.4–206.6 | 0.9973 |
| `5cf9f8b8` | 2026-11-24T04:00Z | 777 | 1172.84 | 1172.84 | 30.69 | 1084.0–1198.3 | 1.0000 |
| `c3e28b2c` | 2026-11-24T04:25Z | 641 | 182.96 | 182.96 | 28.71 | 145.5–188.6 | 1.0000 |
| `ebb43992` | 2026-11-24T08:02Z | 667 | 531.96 | 532.46 | 23.79 | 482.2–545.1 | 0.9991 |
| `49fbe32c` | 2026-11-24T08:33Z | 323 | 458.41 | 458.41 | 28.23 | 407.7–469.6 | 1.0000 |
| `249f8169` | 2026-11-24T10:25Z | 463 | 280.65 | 280.65 | 24.52 | 242.3–288.3 | 1.0000 |
| `9431babe` | 2026-11-24T10:55Z | 403 | 445.14 | 445.14 | 29.93 | 393.4–456.0 | 1.0000 |
| `571e405b` | 2026-11-24T12:00Z | 657 | 670.46 | 670.46 | 10.44 | 626.0–685.9 | 1.0000 |
| `1a1bed81` | 2026-11-24T12:15Z | 928 | 866.19 | 866.19 | 23.38 | 799.7–885.5 | 1.0000 |
| `5f069aea` | 2026-11-24T14:00Z | 268 | 318.60 | 318.60 | 21.58 | 281.2–327.0 | 1.0000 |
| `a6f0dc23` | 2026-11-24T16:00Z | 522 | 367.63 | 367.63 | 29.40 | 320.3–377.0 | 1.0000 |
| `07ffb456` | 2026-11-24T16:25Z | 747 | 584.25 | 584.25 | 8.09 | 546.4–597.9 | 1.0000 |

- **Reproduce.**
  - **The rows.** `kubectl --context hetzner-prod -n bex-system exec bex-db-1 -c postgres -- psql -U postgres -d bex` with `SET default_transaction_read_only = on`, then: `sandbox_running_intervals WHERE NOT usage_billed AND started_at >= '2026-11-23T17:00Z'`, joined to the `usage_hourly` rows of each `sandbox_id` (`resource_kind = 'sandbox'`) and to `agent_session_sandboxes`.
  - **The counter.** `kubectl --context hetzner-prod -n monitoring exec <prometheus-server pod> -c prometheus-server -- wget -qO- 'http://localhost:9090/api/v1/query?query=sandbox_pod_cpu_usage_seconds_total{namespace="…",pod="…"}[<running s + 390>s]&time=<ended_at + 90 s>'`. The API server's service proxy cannot reach the pod: `i/o timeout`.
- **Next (worker):** t003, the cutover at one UTC hour, then t004 and t005. t006 waits for production to run the cutover.
