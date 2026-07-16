# Service pages

The service walk used a Git-backed web service for the primary comparison, plus a Git-backed static site and Git-backed cron for type-specific settings. A local image-backed cron was retained only to confirm its intentionally smaller source configuration.

## Page-by-page verdicts

| Page | Live Render evidence | bex evidence | Verdict | Disposition |
| --- | --- | --- | --- | --- |
| Service root | Deploy history is the main content; Events is a separate tab. `render-walk-service-overview.png` | The root redirects to Deploys; Events remains a direct tab. | Match | Closed by `w5/m36` on 2026-07-15. |
| Settings | General, Build, Deploy, domains, networking, notifications, health, maintenance, and danger-zone controls | Equivalent supported controls are present; unsupported Shell, disk, preview, and drain families are explicit non-goals | Match for supported scope | Not a gap. `render-walk-service-settings.png` / `bex-walk-service-settings.png`. |
| Environment | Variables, secret files, group linking, add/edit actions, and Export | Equivalent controls; Export downloads deterministic dotenv only after every current value is freshly revealed, and fails closed otherwise | Match with stricter export safety | Closed by `w5/m36` on 2026-07-15. Local secret-store unavailability remains an honest configured degradation. |
| Logs | Search, range selector, display actions, and log stream | URL-owned 30m–1d history ranges, structured type/level/method/status/path/instance filters, search, and explicit live tail | Match; bex has extra structured filters | Closed by `w5/m36` on 2026-07-15. Host/path store support remains owned by `w3/m18`. `render-walk-service-logs.png` / `bex-walk-service-logs.png`. |
| Metrics | Time range, event filter/timeline, application CPU/memory/instances, and request/network controls | Equivalent selected-range event timeline/category filter plus CPU/memory/instances, request percentile/status/grouping, and network charts | Match for supported metrics | Closed by `w5/m36` on 2026-07-15. Functional host/path metrics remain owned by `w3/m18`. `render-walk-service-metrics.png` / `bex-walk-service-metrics.png`. |
| Events | Separate event feed with lifecycle entries | Separate event feed with lifecycle entries and honest empty/degraded states | Match | Not a gap. Event-detail fidelity remains owned by `w3/m16`. |
| Deploy list | Status-filterable deploy history with manual deploy/rollback actions | Deploy history, status, commit, and actions | Match | Not a gap. |
| Deploy detail | Deploy identity, lifecycle status, source, timestamps, and logs | Exact deploy route, truthful timeline, shared actions, and build-log pane | Match | Not a gap. Local control-plane unavailability was displayed honestly. `render-walk-deploy-detail.png` / `bex-walk-deploy-detail.png`. |
| Scaling and plan | Instance type/scaling controls live in service settings/scaling | Scaling and Plan are split into explicit tabs with the equivalent supported controls | Functional match; information architecture differs | Not a gap. `render-walk-service-scaling.png` / `bex-walk-service-scaling.png`. |

## Static-site verdict

Render gives Redirects/Rewrites and Headers dedicated routes. bex puts the equivalent ordered add/edit/save editors in the static site's Settings page. The route split is information architecture only; the supported behavior is present. This under-claim changed ADR018's Header-rules UI cell from partial to complete.

Evidence:

- Render: `render-walk-static-settings.png`, `render-walk-static-redirects.png`, `render-walk-static-headers.png`.
- bex: `bex-walk-static-settings.png`.

## Cron verdict

The like-for-like Git cron comparison includes schedule, command, and build and deploy configuration on both products. The bex image-backed fixture correctly omits Git build controls; that is source-type behavior, not a parity gap.

Evidence:

- Render: `render-walk-cron-overview.png`, `render-walk-cron-settings.png`.
- bex: `bex-walk-git-cron-overview.png`, `bex-walk-git-cron-settings.png`.

## Explicit not-gaps

- Shell/browser exec, one-off jobs, persistent disks, log drains, workflows, and preview-environment families remain governed by [`.pm/DO_NOT_DO.md`](../../../.pm/DO_NOT_DO.md).
- Combining static route/header editors in Settings instead of dedicating two tabs does not remove behavior.
- Bex's richer structured-log filters and separate Plan tab are acceptable supersets.
