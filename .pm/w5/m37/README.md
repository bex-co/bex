# w1 · m37 — Maintenance mode: user-toggled interstitial + custom page

**Worker:** worker1 **Goal:** Render's `maintenanceMode {enabled, uri}` exists for bex web services: a tenant can take a service intentionally offline behind a maintenance page (default or their own) without suspending it, toggled from any surface. **Status:** todo

## Tasks (in order)

| id   | title                                                    | est | depends_on |
| ---- | --------------------------------------------------------- | --- | ---------- |
| t001 | Capture Render's maintenance-mode serving semantics       | 30m | —          |
| t002 | CRD: `maintenanceMode {enabled, uri}`                     | 30m | t001       |
| t003 | Operator: route hosts to a maintenance responder          | 60m | t002       |
| t004 | Custom-page `uri` support                                 | 40m | t003       |
| t005 | REST/GraphQL/MCP with Render's object shape               | 40m | t002       |
| t006 | Settings toggle + service-header banner                   | 40m | t005       |
| t007 | Render parity                                             | 30m | t004, t006 |
| t008 | Simplify                                                  | 30m | t007       |
| t009 | Test coverage                                             | 45m | t007       |
| t010 | Closeout                                                  | 15m | t009       |

## Definition of done

Enabling maintenance mode on a web service makes every host it serves (platform + custom) answer with the maintenance page at the captured status code while the app pods keep running untouched; a non-empty `uri` serves the tenant's page per the capture; disabling restores normal serving without a deploy; the `{enabled, uri}` object round-trips with Render's exact shape on REST/GraphQL/MCP; the dashboard has the toggle and shows an in-maintenance banner on the service header. Suspend/resume and auto-sleep interactions are defined and tested, not accidental.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker` round 6, 2026-07-14 — field-level spec-grep of Render's live OpenAPI (`maintenanceMode`: `{enabled: boolean, uri: string}` on `webServiceDetails` + POST/PATCH; `uri` = "the page to be served when maintenance mode is enabled", linking render.com/docs/maintenance-mode). Zero hits in `lego/`.
- **NOT the ledger's non-goal:** ADR018 marks "Maintenance runs" `—`, but that row is Render's `/maintenance` **managed-infra** surface (platform-scheduled runs). This field is the **tenant-facing toggle** — a different capability the row-level audit never inventoried. Recorded here so the anti-goal screen isn't misread.
- **Goal linkage:** Render parity (service contract) + GOAL #1's lifecycle verbs — the missing state between "serving" and "suspended" (offline-with-a-page, pods intact).
- **Expected outcome:** deliberate downtime stops looking like an outage; one fewer permanent allowlist entry for `w7/m30`'s conformance suite.
- **Why now:** verified-in-spec gap; the serving-plane seams it needs (Traefik middleware or the static-server default-page path, the activator's interstitial precedent) all exist; w1 is at two actionable milestones. Render parity task included — all-surface change.
