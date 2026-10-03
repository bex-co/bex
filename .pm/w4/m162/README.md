# w4 · m162 — Keep platform health checks off tenant HTTP paths

**Worker:** worker4 **Goal:** `/healthz` on a tenant hostname follows the same hosting contract as other application paths: suspended services report suspension, sleeping services wake, and running static sites apply their content and rules. Platform readiness remains independent of tenant HTTP paths. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on | status |
| --- | --- | --- | --- | --- |
| t001 | Audit the two public responders and readiness consumers | 20m | — | todo |
| t002 | Remove the activator health-path bypass and preserve platform readiness | 30m | t001 | todo |
| t003 | Remove the static-server health-path override and preserve platform readiness | 30m | t001 | todo |
| t004 | Render parity | 15m | t002, t003 | todo |
| t005 | Simplify | 10m | t004 | todo |
| t006 | Test coverage | 45m | t004, t005 | todo |
| t007 | Closeout | 20m | t006 | todo |

7 tasks, approximately 2h 50m. The two implementation paths have distinct root causes; [finding.md](finding.md) contains the complete live requests, responses, controls, scope, and precedent disposition.

## Definition of done

1. Create an owned Free Go web service from `bex-co/bex`, root `examples/hello-go`, build `go build -o app .`, start `./app`, auto-deploy off. Its running `/`, `/healthz`, `/healthz?qa=r52`, and `/healthz/` return its `OK` body. Suspend in Settings. After reconciliation, `/healthz` and encoded `/health%7a` GET return the same JSON `503 {"error":"service suspended"}`, `Retry-After: 3600`, and `Cache-Control: no-store` as `/`; the query variant with HTML Accept returns the suspended HTML page, and HEAD returns 503 without a body. The Deployment stays at zero; requests do not resume it. Reload Settings and repeat. Resume restores the app's own `OK`.
2. Set that fixture's Idle timeout to 5 min and wait for Sleeping without public traffic. Requests only to `/healthz` (including the query/encoded variants) must initiate the ordinary wake contract: initial retryable 503, then the application's `200 OK` within 60 seconds. They must not produce an empty 200 while the Deployment stays at zero. Confirm Running on the same revision; `/healthz/` remains a working control. The original probe held the service at zero through 117 seconds of health-path requests, then the slash control woke it in under 23 seconds.
3. Create an owned no-build static site from `examples/static-site`, publish `.`, auto-deploy off. With no rules, `/healthz` follows the documented extensionless SPA fallback, just as `/healthz/` does. Save two rewrites: `/healthz → /render.yaml` and `/qa-health-control → /render.yaml`. After resolver refresh and a fresh page load, GET `/healthz` returns the same target file bytes as the control and direct `/render.yaml`, with its normal content/cache headers, instead of an empty platform 200.
4. Suspend that static site. Both `/` and `/healthz` (plus encoded form/HEAD) follow the activator's suspended 503 contract, including Retry-After/no-store. Reload and repeat. Resume restores site content and the saved rewrites. REST/GraphQL/MCP and the dashboard retain their accurate lifecycle and saved-rule state.
5. Delete both owned fixtures through the UI. By-id REST and their public `/healthz` URLs return 404; owned App/child/build/Secret residue is absent. Revoke only the run's QA session.

Platform readiness, maintenance/custom-host coverage and other unprobed classes belong to the implementation/test tasks below, not claims of live acceptance already performed.

## Source + Goal linkage

- **Source:** user-directed continuous `qa-find-bugs`, `muse.env`, w4; sweep 52, 2026-10-03 06:59–07:14 UTC. Two independently traced major defects, full durable evidence in [finding.md](finding.md); screenshots and full local captures listed there.
- **Goal linkage:** [ADR003](../../../docs/ADR003-control-plane.md) free-tier wake, [ADR007](../../../docs/ADR007-restart-suspend-and-resume.md) truthful suspension/wake, and [ADR029](../../../docs/ADR029-static-sites.md) host-specific static content and rules. Supports ADR008's trustworthy hosting and agent/API behavior.
- **Expected outcome:** public health URLs describe the tenant workload, and static routes are not silently intercepted by the platform's own readiness response.
- **Why now:** a suspended workload falsely passes status-only monitoring; a sleeping workload does not wake on these requests; a saved static rewrite is bypassed. Controls distinguish both causes from routing propagation, cache staleness, and broken API state.
- **Render parity included:** public tenant HTTP behavior changes. [Render free services](https://render.com/docs/free) describe request-triggered spin-up, with a specific robots.txt exception; no live Render `/healthz` probe was available. Keep bex's documented bounded 503 wake response and suspension negotiation. Do not broaden this into unrelated parity work.
- **Scope:** schedule fixes only. This QA filing does not implement them. Seven tasks include an explicit shared-scope audit and platform readiness validation; no production rollout is authorized by the filing itself.
