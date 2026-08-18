# w7 · m82 — Build failure classification: retry only what retrying can fix (ADR060 D2 + D4)

**Worker:** worker7 **Goal:** a build fails once when the tenant's input is wrong, retries free when the platform disrupts it, and says which of the two happened. **Status:** todo

## Tasks (in order)

| id   | title                                                                                          | est | depends_on   |
| ---- | ---------------------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Reserved exit codes across clone / native-prepare / buildkit / push / sign                       | 45m | —            |
| t002 | `podFailurePolicy`: absorb disruption, fail tenant errors immediately, retry only the unclassified | 45m | w7/m82/t001 |
| t003 | Classified outcome reaches the App condition, the deploy record, and the metrics                 | 50m | w7/m82/t002 |
| t004 | D4 registry hardening: push retries, conditional TLS verify, Zot `gcDelay` + `scrub`             | 45m | w7/m82/t003 |
| t005 | Infra-success SLO + correlated-failure alert                                                     | 40m | w7/m82/t004 |
| t006 | Render parity sweep: build/deploy failure reason across REST · GraphQL · MCP · dashboard          | 30m | w7/m82/t005 |
| t007 | Simplify the code this milestone changed                                                         | 30m | w7/m82/t006 |
| t008 | Test coverage for the shipped behavior                                                           | 40m | w7/m82/t006 |
| t009 | Closeout                                                                                         | 15m | w7/m82/t007, w7/m82/t008 |

## Definition of done

A `clone` exit 128 and a `buildkit` exit 1 each fail **once**, attributed to the user, with no second attempt. A build pod killed by node drain or preemption retries automatically without consuming the tenant's attempt budget and without failing the deploy. OOM never retries. The deploy's failure reason distinguishes the classes across REST, GraphQL, MCP and the dashboard. Infra-success rate is a queryable series. Every leg is pinned by tests that fail against today's flat classification.

## Source + Goal linkage

- **Source:** [`.pm/w7/builder-issues.md`](../builder-issues.md) §3.1, §3.2, §3.8 (P0/P1/P7) — measured on hetzner-prod 2026-08-17; [docs/ADR060](../../../docs/ADR060-build-worker-reliability-and-performance.md) D2 + D4 (rollout step 3, "independently shippable"); `.pm/w1/046.md` F11.
- **Goal linkage:** [`.pm/GOAL.md`](../../GOAL.md) #3 (git push to deploy) — a deploy that fails for a reason the tenant cannot act on is the worst failure mode a PaaS has.
- **Expected outcome:** wasted retries stop; infra-caused failures become invisible to tenants; tenant-caused ones become unambiguous and fast; a build SLO becomes definable because `user_failed` can finally be excluded from it.
- **Why now:** measured on production the same day this milestone was written — of the six most recent `bex-build` pods, two distinct failing builds each produced **two** pods, spending a doomed second attempt on a deterministic failure while holding a node-exclusive 7Gi slot for its full duration. Both surfaced as the same flat `BuildFailed` as a platform fault would. D1 (`w2/m72`) already made build state a first-class status machine; classification is what makes the outcome metric it ships mean anything.
- **Render parity task included:** yes — the classified reason reaches the deploy record that REST, GraphQL, MCP and the dashboard all surface (`m79`'s `failure_reason` path), so the four surfaces must agree on vocabulary and error shape.
