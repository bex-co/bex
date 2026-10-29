# Incoming streaming requests are invisible until the response ends (a664)

Why: Free web services terminate a user's first HTTP response before a full idle timeout has elapsed since that incoming request.

**Severity:** major. Live dashboard QA on 2026-10-15 local / 2026-10-16 UTC, user-selected w4. Extend m164 with t008; ship the filing only. [Durable requests, responses, streamed frames, fixture and client source](stream-request-start-a664.json).

## Reproduce and measured result

Create two owned Free Existing Image web services using `node:24-alpine`, port 8080 and the fixture source in the evidence JSON. `/events` flushes HTTP 200 immediately, sends a marked SSE heartbeat every 15 seconds and ends normally after 380 seconds. Do not request any other public path during each stream. Read status through the dashboard/API and owned App/Deployment/pod observations.

| Observation (UTC) | A: native Settings 5 min | B: documented API extension 120 seconds |
| --- | --- | --- |
| Service | `srv-db8pasi9cl7s73ae46c0` | `srv-db8perfdbg8c73dgh8fg` |
| Initial last-active stamp | 03:09:48 | 03:18:16 |
| Accepted idle setting | 03:11:21.662, SetIdleTimeout 300 | 03:18:55.064, QaA664RepeatIdle 120 |
| Actual incoming request start (Traefik log) | 03:12:13.873032724 | 03:19:30.455815648 |
| First HTTP 200 headers (client) | 03:12:14.023 | 03:19:30.643 |
| Ready=False / AutoHibernated | 03:14:48 | 03:20:16 |
| Time from incoming request to hibernation | 154.1 seconds, below 300 | 45.5 seconds, below 120 |
| Client aborted | 03:15:28.271, 13 heartbeats | 03:20:56.217, 6 heartbeats |
| Time from client start to abort | 194.877 seconds | 86.324 seconds |

Both App messages explicitly report idle ≥ configured TTL, the stamp remains the initial Running value, and Deployment replicas become zero. Pod termination grace ends at 03:15:28 / 03:20:56, matching stream aborts. Each request-log query returns exactly one HTTP 200 `/events` entry with `hasMore:false`; completed durations are 194315962896 / 85673515170 ns. There is no intervening restart, deploy or suspension. Clients have a 25-second inactivity timeout and a 410-second overall timeout; the last heartbeats precede abort by less than 15 seconds, errors are empty, and neither receives `done`. Thus the clients' timeouts did not cause the failure.

Fresh desktop Settings shows Sleeping and 5 min. Fresh 390px Settings on B shows Sleeping and the correct custom 2 min option; document width is 390. Screenshots, inspected locally: `.playwright-mcp/qa-a664-a-hibernated.png` and `.playwright-mcp/qa-a664-b-hibernated-mobile.png`. B's timeout was deliberately set via the documented positive-TTL API extension, not a native picker option.

**Target:** an incoming request that reaches the awake service advances activity before its response finishes. A cannot auto-hibernate before 300 seconds after that request; B cannot before 120 seconds. This does not assert that outgoing-only SSE heartbeats renew the clock forever or that a 380-second stream must complete after a shorter quiet window.

## Cause and implementable correction

- `lego/operator/internal/controller/activity.go:132–156` reads completed `traefik_service_requests_total` plus the two WebSocket byte counters, with the existing first-positive/current-instant protections. It has no generic HTTP request-start input. `recentlyActive:169–189` therefore sees no eligible activity for a service's first, still-open HTTP response. `app_controller.go:2379–2387,2814–2816` expires the original stamp and selects zero replicas.
- The repository pins Traefik chart 41.0.0 in `deploy/helm-artifacts.lock:22`; the downloaded official chart's SHA256 matches the lock and its appVersion is v3.7.5. Opened the actual pinned [middleware](https://raw.githubusercontent.com/traefik/traefik/v3.7.5/pkg/middlewares/metrics/metrics.go): NewServiceMiddleware at 80–91 binds service counters; ServeHTTP at 132–185 increments the ordinary request counter only after the downstream handler returns. This matches the captured stream timing. This is source-based cause attribution, not a claim that raw live Prometheus samples were captured.
- The existing TLS-specific request counter increments before the downstream call. The pinned [Prometheus producer](https://raw.githubusercontent.com/traefik/traefik/v3.7.5/pkg/observability/metrics/prometheus.go) at 20–52,221–232 confirms `traefik_service_requests_tls_total` and its service/TLS labels. It is an existing useful primitive, but covers TLS only. `deploy/gitops/base/values/traefik.values.yaml:159–166` enables service metrics and explicitly retains plain HTTP without forced HTTPS redirect. A TLS-only fix leaves the same mechanism on exposed HTTP routes.
- Preferred complete correction: add a bounded per-App HTTP-start counter to the existing `websocketegress` plugin, incremented once on handler entry for HTTP and HTTPS before forwarding. Reuse its immutable App identity, existing App cap and listener; do not add path/client labels, a new metrics server or a global open-connection keep-awake gate. Ordinary HTTP currently passes straight through at `deploy/gitops/charts/traefik-plugins/websocketegress/websocketegress.go:106–111`; existing maps/cap and `metricsBody:295` supply the extension points. Preserve streaming interfaces and both WebSocket byte counters.
- Export the new signal, explicitly allow it through `deploy/gitops/base/prometheus.yaml:364–381` (the 9101 job currently drops names outside its WebSocket regex), and feed it into the same per-series activity expression in activity.go. Preserve completed-request compatibility, first-positive detection, current-step handling, reset safety, finite quiet expiry, fail-awake reads and optimistic stamp conflicts. Do not change billing/request-count API semantics. Adding only an operator query cannot create a missing counter.
- The plugin ConfigMap uses `disableNameSuffixHash:true`; the implementation must arrange and verify an actual Traefik rollout/load of changed plugin code, not assume the source ConfigMap update hot-reloads code. No production platform rollout was performed by QA.

## Blast radius and controls

The production census finds one `recentlyActive` call (`desiredReplicas`) and one constructor wiring (`lego/operator/cmd/manager/main.go:68`). The fix applies globally within existing auto-sleep eligibility: Free/untiered web services and the legacy empty-type web alias, unsuspended only. Paid web, static sites, cron, workers, private services, Postgres and Key Value must retain their present eligibility. These sibling paths were traced in code, not exercised live in this pass. Image/Git/native/Blueprint create paths converge on App reconciliation; only image-created web fixtures were exercised here.

The existing middleware is attached by `reconcileIngressWithMiddlewares:2904–2920` via the single enabling call at 2909 and its constructor at 3505. Two explicit disabling calls at 3865/3892 cover suspended/published static-site paths; do not attach a new plugin to those just to fix web sleep. Keep activator readiness/wake behavior and genuine Hibernated reporting. Before a start counter has been scraped there remains the documented final-unscraped-interval limitation; this reproduction leaves many scrape intervals before hibernation, so that limitation cannot explain either failure.

Keep m164's original sparse/steady HTTP, current-instant and m151/m161 controls. Re-run blocking-response producer tests and real PromQL evaluation with a start-only signal while completed traffic remains absent/flat, mixed per-pod resets, other-App isolation, both protocols and eventual idle expiry. The new source requires renewed Simplify and Test coverage; their 2026-10-03 completion applies to the older three-signal implementation only.

## Dedupe, parity and limits

Reviewed open/blocked/done records across workstreams, latest 40 commits, and targeted activity history: m164 already owns this quiet-window guarantee, so this extends it rather than creating another inbox note. Its landed first-positive/current-step correction still reads completed responses on current main; this request-start gap is not fixed code awaiting rollout. Preserve the original m151 and m161 acceptance coverage; this pass does not re-prove default-900s or WebSocket controls. Read DO_NOT_DO: actual inbound request loss is distinct from the excluded scanner/self-wake framing.

[Render Free documentation](https://render.com/docs/free) describes inactivity by incoming HTTP/WebSocket traffic. That supports request-start activity; it does not establish indefinite keep-awake from outgoing SSE bytes. No API shape or idle policy expansion is proposed. REST/GraphQL/MCP/dashboard sleep/wake and timeout consistency remain t004 verification work; full MCP parity was not exercised here.

Unverified live: raw Prometheus counter series, exact running Traefik binary version, plain HTTP streaming, custom domains, HTTP/2, other resource types, default timeout and scrape outages. Three dashboard GraphQL polls logged transient ERR_CONNECTION_CLOSED, not independently reproduced; they are recorded separately and not filed as another defect. The public Node clients are independent of those dashboard polls.

## Cleanup

Native DeleteService succeeds for B at 03:27:05.281 and A at 03:27:10.705. Both REST detail reads return 404 and all seven baseline resource ID sets match at 03:28:00.425; both public endpoints return 404, App objects are absent and owned pod counts are zero. Exact-run logout exits zero; browser whoami is 401, cookies zero, one about:blank page. Own handles and the stat-matched shared handle were removed. No foreign resources, paid plans or product code changed.
