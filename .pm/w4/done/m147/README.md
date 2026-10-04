# w4 · m147 — Refresh service activity while the page stays open

**Worker:** worker4 **Goal:** newly recorded service events appear on the open Activity page and completed deploys lose their in-progress state and Cancel action. **Status:** done 2026-10-02 — t001–t006 complete; hosted acceptance passed

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Refresh the activity head with a bounded moving window — **DONE** | 40m | — |
| t002 | Preserve paging and shared Events/Metrics callers — **DONE** | 35m | t001 |
| t003 | Render parity — **DONE** | 15m | t002 |
| t004 | Simplify — **DONE** | 10m | t003 |
| t005 | Test coverage — **DONE** | 30m | t003 |
| t006 | Closeout — **DONE** | 15m | t004, t005 |

## Definition of done

Repeat [the captured live probes](finding.md) with a new disposable free web service.

- Open Events during its first deploy, keep the document visible, and wait for the API's deploy_ended event. Within the existing 30-second resource refresh baseline after that event is available, the page contains the build/deploy lifecycle rows, shows the completed deploy as Live, and removes the started row's In Progress badge and Cancel action without reload.
- Reload Events once as a control, then set idleTTLSeconds to 60 through the captured GraphQL mutation while leaving the page open. The newly recorded idle_timeout_changed and service_hibernated rows appear within that same baseline after API availability. Wake the sleeping service with the captured public request/retry sequence; its service_woken event also arrives without navigation. A subsequent fresh load contains the same retained event IDs.
- Replay the fixed historical-window and current bounded-window requests. The historical query still excludes later events; the current query includes them. REST, GraphQL and both MCP event tools retain correct event identity and timestamps. Do not remove the backend range cap to make a moving UI query pass.
- Open Metrics, choose Last 30 minutes and show Event timeline. Its queries advance the selected window at the existing 15-second cadence and collect a later event, as the working control did in this sweep.
- Delete the fixture through the dashboard, confirm its REST read is 404 and its exact App/workloads are gone, then revoke the QA session.

Older-page accumulation, hidden-tab behavior, absolute Metrics ranges and sibling resource types were not live-exercised here; their verification is explicit work in t002/t005 rather than an observed-failure claim.

## Source + Goal linkage

- **Source:** continuous qa-find-bugs, 2026-10-02 sweep 7; muse.env login; user-directed w4 placement. [Finding](finding.md) includes complete requests/responses, two page lifetimes, dependency/source trace, aliases and prior-DoD disposition.
- **Goal linkage:** ADR008 reliable hosting; ADR006 one event service behind the adapters; ADR018 service events; docs/render-artifacts/service-events.md's retained, truthful timeline contract.
- **Expected outcome:** a user watching a deploy or service transition sees its recorded outcome without reloading; historical paging and selected Metrics ranges remain usable.
- **Why now:** the header reached Running/Live while Activity retained only Deploy started, In Progress and Cancel for over two minutes after the deploy completed. A fresh page then missed subsequent configuration and hibernation events. These are present in every probed API surface.
- **Sizing:** 145m across bounded refresh, shared paging/range semantics and meaningful temporal regression tests plus standing closing tasks. This is more than adding a pollInterval: a frozen endTime still excludes new rows, and advancing only that end exceeds the 720-hour cap.
- **Render parity included:** this changes a tenant UI surface. Official Render docs support bounded/cursor event reads and visible deployment history; this hunt did not measure Render's authenticated refresh cadence. The 30-second target comes from Bex's existing resource polling policy.

## Dedupe and limits

No open milestone covers this after scanning 36 open/blocked milestone READMEs, inbox notes and completed history. w4/073 fixed the Deploys list; this route does not use that hook. w3/m19 supplied historical windows/cursors, w6/m122 fixed catalog filtering, and w7/m66 supplied lifecycle facts. Their producer/history guarantees are preserved; this is an uncovered live freshness gap, not evidence those source fixes regressed. The finding walks their relevant complete DoDs. Product code is unchanged by this filing.

## Blocked closeout — 2026-10-02

The release pipeline must deploy the dashboard; QA must then replay the live deploy/sleep/wake, API identity/range and Metrics controls, delete its fixture and revoke its session. All implementation tasks and local checks passed; see [verification](verification.md).

## Live acceptance — 2026-10-02

Production dashboard carried fix `fb9bc40db` (ancestor of deployed `18958459c`). Fixture `qa-20261002-w4x-activity` (`srv-db0aehitm2ss7389qoeg`), free Go web from `bex-co/bex@main` `examples/hello-go`, `go build -o app .` / `./app`, port 3000, auto-deploy off. Times UTC 2026-10-03; the Events document stayed visible (`document.visibilityState=visible`) without navigation between samples.

- **First deploy:** Events opened 06:57:23 with one row (Deploy started / In Progress / Cancel). API `deploy_ended` (`evt-vn1fnroq59t271an3uf8`) first returned at 07:01:13; the first page sample at 07:01:18 had four rows, "Deploy ended — Live", no In Progress badge and no Cancel — no reload.
- **Control reload** 07:01:30: four rows.
- **Idle timeout:** captured `SetIdleTimeout(idleTTLSeconds:60)` at 07:01:35 → `idle_timeout_changed` `evt-mf2r6cfi24a9bd5fj9l0` 07:01:36 (row present by 07:02:14) and `service_hibernated` `evt-42ukemco6n4h580er5qv` 07:02:02, available by 07:02:08, on the page at 07:02:29 (≤21 s).
- **Wake:** public request at 07:02:40 → 503 `{"error":"service hibernated","retryAfter":5}` with `retry-after: 5`; retries every 5 s; 200 `OK` at 07:03:15. `service_woken` `evt-fqpuv4om6hg7j5tqslkg` (07:03:27, plus `server_available`) available by 07:03:29, on the page by 07:03:59 (between the 07:03:53 and 07:03:59 samples, ≤30 s).
- **Fresh load:** the reload's `ServiceEvents` response held the same eight IDs as REST `GET /v1/services/{id}/events`.
- **Windows:** the fixed historical window 2026-09-03T06:57:20Z → 2026-10-03T06:57:20Z still returns only `evt-lmkunef6s4aiqrua85ir` (deploy_started 06:57:10Z) on GraphQL, REST, MCP `list_service_events` and MCP `list_events`; a 720-hour window ending now returns all nine events on each. Extending only the end time still fails "query range exceeds 720 hours" (cap kept).
- **Metrics:** Last 30 minutes + Show event timeline issued `ServiceEvents` at 07:07:37, 07:07:52 and 07:08:06 (15 s apart), each moving both bounds forward by 15 s; the count rose from 10 to 11 when a later `service_hibernated` (`evt-s0qmrhqnu5hlktc9s8kf`) arrived.
- **Cleanup:** deleted through Settings → Delete Service (typed sudo confirmation); REST GET → 404; read-only kubectl finds no `w4x` App, Deployment, Service, Ingress, Pod, Secret or build Job. QA session revoked at the end of the run.
