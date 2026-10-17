# w4 · m224 — Refresh Metrics status choices while the page stays open

**Worker:** worker4 **Goal:** newly observed HTTP status codes become selectable on an already-open Metrics page. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Refresh combined Metrics discovery from one polling owner | 35m | — |
| t002 | Audit both discovery consumers and their shared route aliases | 25m | w4/m224/t001 |
| t003 | Render parity | 15m | w4/m224/t002 |
| t004 | Simplify | 15m | w4/m224/t003 |
| t005 | Test coverage | 35m | w4/m224/t003 |
| t006 | Closeout | 20m | w4/m224/t004, w4/m224/t005 |

Six tasks,145m total (2h25m), including the shared-observer audit, behavioral coverage and two-breakpoint live acceptance. Minor functional discovery defect. This ships the filing; no product code is changed.

## Definition of done

Replay the owned Free web journey in [finding.md](finding.md) at fresh390×844 and1440×1000 using the recorded Node command and eight-request sequence. Wait for the actual discovery API to index a value before judging UI freshness.

- On mobile, mount Metrics after the initial six requests, set Last30minutes and Path=/qa-a875-batch, and send the recorded204 request. Once a healthy API read includes204, the mounted Status Code picker offers204 within the next successful visible30s discovery poll, without a reload. The path-scoped total becomes7.
- On desktop, mount with204 already discoverable, select404 and the same path, then send409. Once the API includes409, the mounted picker offers409 within the next successful visible30s discovery poll. The existing404 selection and its count2 remain until an explicit choice. Selecting409 yields1; All yields8. Request logs contain exactly the eight controlled method/status pairs.
- Background discovery preserves the current picker, focus, committed and uncommitted Path inputs, Host, status, grouping, percentile and ready charts. Existing exact-path, GET/HEAD/POST grouping,4xx and Host+Path controls keep their recorded counts. There is one combined INSTANCE+STATUS_CODE discovery owner for a mounted resource, including when both cards observe it; no independent second timer is added.
- Complete t002/t005's source-traced compatibility checks: retained and historical instance choices remain selectable; a transient discovery failure keeps the last usable answer for that resource; a resource change cannot reuse another resource's values; hidden tabs skip polls and resume on a visible tick. These are implementation/test requirements, not failures claimed live-observed by a875. Cold loading keeps today's class presets and structural pending layout.
- Relevant dashboard checks and the live replays pass; delete each new fixture and its exact cluster objects, restore all seven inventory-ID sets, and revoke only the run's QA session before closeout.

## Source + Goal linkage

- **Source:** continuous functional qa-find-bugs w4, a875,2026-10-17 UTC; muse.env consumed privately by the login helper. [Finding and research](finding.md), [complete requests, responses, observations, probe sources and cleanup](evidence.json). Research main=c685245f5e76360f855887cbaefb32e4eb7d7722; the inspected discovery hook is unchanged from the live hunt's starting7fa064b89.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) dependable hosting and [ADR010](../../../docs/ADR010-observability.md) actionable operational metrics. A newly returned error code should be isolatable while a tenant investigates current traffic.
- **Expected outcome:** the chart and its available exact-status choices reflect successful recent reads together, without requiring a page reload or losing diagnostic selections.
- **Why now:** two healthy, independent post-mount discoveries fail at fresh mobile and desktop widths. The existing Logs freshness repair uses another hook; its network Host consumer does not refresh this Metrics document.
- **Scope and lineage:** keep the combined-query/no-duplicate behavior of [w4/m100](../done/m100/README.md) and selection retention of [w5/m91](../../w5/done/m91/README.md). This is a previously unimplemented freshness residual, not evidence that either milestone's unrelated guarantees regressed. [w4/178](../done/178.md) owns the separate Logs discovery and passed its production acceptance in a876; Metrics discovery remains open here. See the complete gap dispositions in [finding.md](finding.md#deduplication-and-original-guarantees).
- **Render parity included:** UI metrics filtering changes. Render documents exact-status filtering and instance selection, but its authenticated dropdown timing was not measured;30s follows Bex's existing resource cadence. REST/GraphQL/MCP wire semantics stay compatible, and no backend repair is indicated by these healthy replies.

## Limits

One Free web instance was live exercised. Static, worker, private and cron routes, historical or replaced instances, transient discovery errors, hidden-tab behavior and resource-to-resource navigation are source/test audit work. This run injected no response, delay or transport failure. It does not certify the full count-boundary, loading-geometry or datastore-metrics contracts tracked elsewhere.
