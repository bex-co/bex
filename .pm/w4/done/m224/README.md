# w4 · m224 — Refresh Metrics status choices while the page stays open

**Worker:** worker4 **Goal:** newly observed HTTP status codes become selectable on an already-open Metrics page. **Status:** done 2026-10-29 — live DoD replayed on pin `d66f96c06991`: a mounted Status Code picker offers a newly seen code (204 on 390, 409 on 1440) on its next background discovery poll, without a reload, and keeps the current selection and Path (loopx w4).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Refresh combined Metrics discovery from one polling owner — **DONE** | 35m | — |
| t002 | Audit both discovery consumers and their shared route aliases — **DONE** | 25m | w4/m224/t001 |
| t003 | Render parity — **DONE** | 15m | w4/m224/t002 |
| t004 | Simplify — **DONE** | 15m | w4/m224/t003 |
| t005 | Test coverage — **DONE** | 35m | w4/m224/t003 |
| t006 | Closeout | 20m | w4/m224/t004, w4/m224/t005 |

Six tasks,145m total (2h25m), including the shared-observer audit, behavioral coverage and two-breakpoint live acceptance. Minor functional discovery defect. Implemented 2026-10-17; awaiting the deployed-build replay (t006).

## Definition of done

Replay the owned Free web journey in [finding.md](finding.md) at fresh390×844 and1440×1000 using the recorded Node command and eight-request sequence. Wait for the actual discovery API to index a value before judging UI freshness.

- On mobile, mount Metrics after the initial six requests, set Last30minutes and Path=/qa-a875-batch, and send the recorded204 request. Once a healthy API read includes204, the mounted Status Code picker offers204 within the next successful visible30s discovery poll, without a reload. The path-scoped total becomes7.
- On desktop, mount with204 already discoverable, select404 and the same path, then send409. Once the API includes409, the mounted picker offers409 within the next successful visible30s discovery poll. The existing404 selection and its count2 remain until an explicit choice. Selecting409 yields1; All yields8. Request logs contain exactly the eight controlled method/status pairs.
- Background discovery preserves the current picker, focus, committed and uncommitted Path inputs, Host, status, grouping, percentile and ready charts. Existing exact-path, GET/HEAD/POST grouping,4xx and Host+Path controls keep their recorded counts. There is one combined INSTANCE+STATUS_CODE discovery owner for a mounted resource, including when both cards observe it; no independent second timer is added.
- Complete t002/t005's source-traced compatibility checks: retained and historical instance choices remain selectable; a transient discovery failure keeps the last usable answer for that resource; a resource change cannot reuse another resource's values; hidden tabs skip polls and resume on a visible tick. These are implementation/test requirements, not failures claimed live-observed by a875. Cold loading keeps today's class presets and structural pending layout.
- Relevant dashboard checks and the live replays pass; delete each new fixture and its exact cluster objects, restore all seven inventory-ID sets, and revoke only the run's QA session before closeout.

## Source + Goal linkage

- **Source:** continuous functional qa-find-bugs w4, a875,2026-10-17 UTC; muse.env consumed privately by the login helper. [Finding and research](finding.md), [complete requests, responses, observations, probe sources and cleanup](evidence.json). Research main=c685245f5e76360f855887cbaefb32e4eb7d7722; the inspected discovery hook is unchanged from the live hunt's starting7fa064b89.
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md) dependable hosting and [ADR010](../../../../docs/ADR010-observability.md) actionable operational metrics. A newly returned error code should be isolatable while a tenant investigates current traffic.
- **Expected outcome:** the chart and its available exact-status choices reflect successful recent reads together, without requiring a page reload or losing diagnostic selections.
- **Why now:** two healthy, independent post-mount discoveries fail at fresh mobile and desktop widths. The existing Logs freshness repair uses another hook; its network Host consumer does not refresh this Metrics document.
- **Scope and lineage:** keep the combined-query/no-duplicate behavior of [w4/m100](../../done/m100/README.md) and selection retention of [w5/m91](../../../w5/done/m91/README.md). This is a previously unimplemented freshness residual, not evidence that either milestone's unrelated guarantees regressed. [w4/178](../../done/178.md) owns the separate Logs discovery and passed its production acceptance in a876; Metrics discovery remains open here. See the complete gap dispositions in [finding.md](finding.md#deduplication-and-original-guarantees).
- **Render parity included:** UI metrics filtering changes. Render documents exact-status filtering and instance selection, but its authenticated dropdown timing was not measured;30s follows Bex's existing resource cadence. REST/GraphQL/MCP wire semantics stay compatible, and no backend repair is indicated by these healthy replies.

## Limits

One Free web instance was live exercised. Static, worker, private and cron routes, historical or replaced instances, transient discovery errors, hidden-tab behavior and resource-to-resource navigation are source/test audit work. This run injected no response, delay or transport failure. It does not certify the full count-boundary, loading-geometry or datastore-metrics contracts tracked elsewhere.

## Implementation record (2026-10-17)

- **t001:** `useMetricsFilterValues` takes `{ poll }`; only NetworkMetricsCard, mounted for every App, passes it, so the combined INSTANCE+STATUS_CODE discovery refreshes from one owner every `RESOURCE_POLL_INTERVAL_MS` and skips hidden tabs (`skipPollWhenHidden`). ApplicationMetricsCard observes the same document and cache entry with no timer. A failed or pending refresh keeps the same resource+field's last answer through a new shared `useLastAnswer` hook (`common/hooks/use-last-answer.ts`), which the Logs discovery (`useLogLabelDiscovery`, w4/178) now also uses; another resource's answer is never shown as current. The Network card keeps a chosen exact status code among its options even if a later answer omits it, so a refresh never silently changes the request filter, and is now keyed by service like the Application card, so a chosen code does not carry into another service. Class presets, cold loading, the chart window and the API are unchanged.
- **t002 census:** two production calls (ApplicationMetricsCard INSTANCE, NetworkMetricsCard STATUS_CODE), one JSX mount of each in ServiceMetricsPage; static sites hide the Application card but always mount the Network owner. `/services/$id/metrics` and `/static/$id/metrics` render the page; `/web`, `/worker`, `/pserv`, `/cron` aliases forward to it. The Application card's union of live, CPU-series and selected instances is unchanged. Postgres/Key Value metrics use separate hooks. Error variants are source-traced; Apollo does not cache a `data: null` error reply, so the observer keeps its data too.
- **t003:** ADR018's core-metrics row records the live discovery refresh.
- **t004:** three /simplify passes (shared `useLastAnswer` extracted from the logs and metrics hooks; card keyed by service; stronger tests).
- **t005:** `use-metrics-filter-values.test.tsx` uses real Apollo with the app's cache and defaults plus fake timers. A newly returned code (and instance) reaches both cards on the next poll from one read; the observer alone never polls; a failed poll keeps the last answer; another resource's answer is never current; hidden tabs skip polls and resume when visible. The two refresh cases fail against the old hook. `network-metrics-card.test.tsx` asserts the card owns the poll and keeps a chosen code a later answer omits. Logs discovery tests pass unchanged on the shared hook. Dashboard suite (5161 tests), lint and typecheck pass.

## Hosted probe — qa pass 91f0c7 (2026-10-19, not the t006 closeout)

A `/qa-find-bugs w4` pass replayed the milestone's core promise on production, pinned to `456090929`, which includes `4df47e1f7`. It used a fresh owned Free web service, `qa-20261019-91f0c7-status` (`srv-dbangea5p3cc73ad8l30`, image `python:3.13-alpine` running `python3 -m http.server $PORT`). The service was deleted afterwards (204 → 404), and only that run's session was revoked.

- **Desktop 1440×1000.**
  - Before the new status: 4×`GET /` (200) and 3×`GET /nope-91f0c7-n` (404). Metrics mounted at 01:54:55Z with Status Code options `All, 2xx, 4xx, 5xx, 200, 404`.
  - 3×`POST /` (501) at 01:55:02Z. `GET /v1/metrics/filters/http` listed `501` by 01:55:05Z.
  - The **still-mounted** picker offered `501` at 01:55:21Z, with no reload.
- **Mobile 390×844.**
  - Fresh mount at 01:55:40Z, options `…, 200, 404, 501`.
  - 2×`GET /etc` (301) at 01:55:41Z. The API listed `301` by 01:55:58Z.
  - The mounted picker offered `301`, sorted in, at 01:56:14Z.
- **Not replayed:**
  - the recorded eight-request Node sequence;
  - path-scoped totals;
  - preservation of an existing selection, focus and inputs;
  - single-owner polling;
  - hidden-tab behavior.

t006 remains the closeout.
