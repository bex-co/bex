# w8 · m15 — Complete outbound-bandwidth accounting: HTTP + WebSocket + direct + datastore TCP

**Worker:** worker8 **Goal:** Replace the HTTP-only `egress_bytes` source with an explicit, loss-detecting composition that covers App HTTP responses, WebSocket downstream frames, App-initiated public traffic, and public Postgres/Key Value responses without charging same-cluster private traffic. **Status:** in progress (implementation + local Cilium validation done; production evidence origin pending)

## Tasks (in order)

| id | title | est | depends_on | status |
| --- | --- | --- | --- | --- |
| t001 | Define the outbound accounting contract and reject lossy diagnostic sources | 30m | — | — **DONE** |
| t002 | Attribute App HTTP response bytes by exact Traefik router | 45m | t001 | — **DONE** |
| t003 | Prototype direct pod-to-world accounting and prove lifecycle/reset semantics | 60m | t001 | — **DONE** |
| t004 | Build the node meter with stable pod and resource attribution | 60m | t003 | — **DONE** |
| t005 | Deploy and scrape the node meter with least privilege and health alerts | 45m | t004 | — **DONE** |
| t006 | Export Postgres proxy backend-to-client bytes by Database | 45m | t001 | — **DONE** |
| t007 | Put public Key Value traffic through a metered SNI front door | 60m | t001 | — **DONE** |
| t014 | Meter WebSocket downstream frames at the public edge | 60m | t001, t002 | — **DONE** |
| t008 | Compose hourly egress collection by resource kind and restart bandwidth evidence | 60m | t002, t005, t006, t007, t014 |  |
| t009 | Validate attribution, exclusions, resets, and no double counting on the Cilium path | 45m | t008 | — **DONE** |
| t010 | Render parity — audit REST, GraphQL, MCP, dashboard, and documented category semantics | 30m | t009 | — **DONE** |
| t011 | Simplify — remove duplicate query and proxy accounting paths | 20m | t010 | — **DONE** |
| t012 | Test coverage — source failures, counter resets, attribution, and traffic matrix | 45m | t010 | — **DONE** |
| t013 | Closeout — DoD met → move milestone to `done/` | 10m | t011, t012 |  |

## Definition of done

For each closed UTC hour, `egress_bytes` is a contiguous, retryable sum of independently observable sources: App HTTP router response bytes, App WebSocket downstream-frame bytes, App direct-to-public bytes, and public managed-datastore response bytes. Same-cluster/private, DNS-to-cluster, control-plane, backup, and dropped traffic are excluded. Source absence or reset is visible and cannot silently become zero; pod churn and node-agent restarts preserve monotonic deltas without re-attributing a reused runtime identity. Tests exercise HTTP, WebSocket, direct TCP/UDP, private service, public Postgres, and public Key Value paths; a live Cilium validation proves attribution and no double counting. Documentation states the byte basis and any remaining Render drift, all exposed usage/metrics surfaces agree, and the bandwidth portion of `w8/001` records a new post-rollout 28-day evidence origin.

## Validation evidence

On 2026-07-15, `scripts/verify-egress-meter.sh` passed on a fresh three-node kind cluster using the production Cilium 1.19.5 datapath settings. Aggregate deltas in the final run were HTTP 4,096 B, WebSocket downstream 4,100 B, direct UDP 544 B, direct TCP 4,372 B, Postgres response 6,031 B, and Key Value response 6,030 B; private and policy-dropped traffic remained zero. Pod replacement, meter restart, deliberate counter-map deletion, and node reboot were exercised. The deliberate deletion exposed a 544 B checkpoint gap through a counter decrease and the reboot proved that durable counter-loss state loaded before recording a second event; both affected hourly windows fail closed even when a restore does not fall below the last Prometheus scrape.

The repository implementation, isolated manual CI workflow, full Go/dashboard suites, GitOps rendering, and Prometheus rule tests are complete. `t008` remains open only for the exact first whole UTC hour after a production rollout; this worktree is intentionally uncommitted and unshipped, so `.pm/w8/001.md` cannot yet contain a truthful new bandwidth evidence origin. `t013` must remain open until that rollout evidence exists.

## Source + Goal linkage

- **Source:** prerequisite split from `.pm/w8/001.md`'s incomplete-egress gate after the 2026-07-14 source audit; `w8/m11` made hourly coverage provable but deliberately left the meter's HTTP-only scope unchanged.
- **Goal linkage:** `GOAL.md` item 5 (usage metering), pillar 1 (one reliable core behind every usage surface), and the tenant-facing Hobby-cap prerequisite in `w8/001`.
- **Expected outcome:** usage reports and future caps measure the documented outbound categories instead of treating HTTP response bytes as all bandwidth, while private tenant traffic remains free.
- **Why now:** the bandwidth cap's 28-day observation window is otherwise collecting the wrong quantity. Completing the source before promoting the cap prevents a month of HTTP-only data from being mistaken for full outbound evidence.
- **Render parity closing task included:** meter values and category semantics are tenant-visible through REST, GraphQL, MCP, and the dashboard even though the response schemas do not change.
