# w6 · m566 — An agent session's sandbox is billed for the CPU it uses and the memory it is allocated, as a Render sandbox is

**Worker:** worker6 **Goal:** an agent session's sandbox pays what a `/v1/sandboxes` sandbox pays for the same running time, used CPU at $0.084 per CPU-hour plus allocated memory at $0.0133 per GiB-hour, instead of the weighted allocated-shape charge. The session's cost and spend ceiling use the same figures. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Record agent sandboxes' used CPU, shown at $0 beside the weighted charge](t001.md) | 1h30m | — |
| t002 | [Check a day of agent CPU readings against the pod counter](t002.md) | 45m | t001 |
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

- **Source:** the recommendation the user approved on 2026-11-22 ("其他的按照你推荐的修"). Since 2026-11-18 a `/v1/sandboxes` sandbox is billed on use ([ADR030 §17](../../../docs/ADR030-pricing.md)), but an agent session's keeps the weighted meter (`UsageBilled: idleTracked(raw)` in `lego/backend/internal/sandbox/meter.go`, and ADR030 §17's "Agent sessions" line). The billing page tells every reader that "a running sandbox is billed for the CPU it uses and the memory it is allocated", which is untrue for the largest sandbox line on it.
- **Evidence (production, 2026-11-22):** November's agent sandboxes in the `bex` workspace billed $231.64 on the weighted meter; their turns ran 21% of the billed hours.
- **Reference:** Render bills a running sandbox for actively used CPU and allocated memory ([ADR104 D8](../../../docs/ADR104-sandbox-render-parity.md), render.com/blog/introducing-render-sandboxes, published 2026-11-04). Devin bills a session's awake time in ACUs and lets it sleep after 5 minutes (docs.devin.ai/admin/billing/usage). With m563 an agent sandbox is awake for little more than its turn, so billing on use and Devin's awake-time model come to nearly the same idle charge. The user approved billing on use.
- **Depends on:** w22/m314, the used-CPU meter for `/v1/sandboxes`. Production already carries it (`dcf1f1991`), and it bills since 2026-11-18. Its last accuracy run (`sandbox-measure.yml`, `leg=usage`) is still pending. t003's cutover waits until that run passes; t001 and t002 do not.
- **Goal linkage:** ADR023 usage metering, ADR030 pricing, ADR047 D6: one billing model for one product's sandboxes ([ADR008](../../../docs/ADR008-vision.md)).
- **Expected outcome:** agent sandbox charges follow use. After m563 removes most idle time, the remaining running time is charged at memory plus the CPU actually used.
- **Why now:** the billing page's copy already promises it, and customers' agent sessions are the largest sandbox line.
- **Render parity:** included: the `cost` fields on REST, GraphQL and MCP, and the billing page.
