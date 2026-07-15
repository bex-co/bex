# w1 · m37 — Maintenance mode: user-toggled interstitial + custom page

**Worker:** worker1 **Goal:** Render's `maintenanceMode {enabled, uri}` exists for paid bex web services: a tenant can take public traffic offline behind a default or custom maintenance page without changing private reachability, configured consistently across API, Blueprint, dashboard, activity, audit, and webhook surfaces. **Status:** done — **DONE** (2026-07-15)

## Tasks (in order)

| id | title | est | depends_on |  |
| --- | --- | --- | --- | --- |
| t001 | Capture Render's maintenance-mode serving semantics | 30m | — | — **DONE** |
| t002 | CRD: `maintenanceMode {enabled, uri}` | 30m | t001 | — **DONE** |
| t003 | Operator: route hosts to a maintenance responder | 60m | t002 | — **DONE** |
| t004 | Custom-page `uri` support | 40m | t003 | — **DONE** |
| t005 | REST/GraphQL/MCP with Render's object shape | 40m | t002 | — **DONE** |
| t006 | Settings toggle + service-header banner | 40m | t005 | — **DONE** |
| t007 | Render parity | 30m | t004, t006 | — **DONE** |
| t008 | Simplify | 30m | t007 | — **DONE** |
| t009 | Test coverage | 45m | t007 | — **DONE** |
| t010 | Closeout | 15m | t009 | — **DONE** |

## Definition of done

Enabling maintenance mode on a web service makes every host it serves (platform + custom) answer with the maintenance page at the captured status code while the app pods keep running untouched; a non-empty `uri` serves the tenant's page per the capture; disabling restores normal serving without a deploy; the `{enabled, uri}` object round-trips with Render's exact shape on REST/GraphQL/MCP; the dashboard has the toggle and shows an in-maintenance banner on the service header. Suspend/resume and auto-sleep interactions are defined and tested, not accidental.

**DoD verified live 2026-07-15** through real Chromium → dashboard → host-run bex-api → CAPD Kubernetes. Enable, confirmation, custom-URL edit, reload persistence, rejected same-service URL, disable, and a second reload all reached the real GraphQL/Core/App-CR path; the invalid save displayed an error and left the CR unchanged. Platform and custom hosts returned the default/custom responses at the pinned statuses while direct private-Service traffic continued reaching the same pod.

The serving audit covered both same-namespace and production-default cross-namespace topology. For an App in `default`, the operator created the App-owned `bex-maintenance-…` `ExternalName` alias to `bex-activator.bex-system.svc.cluster.local:8888`; both Ingress hosts selected it and returned the default 503. Enabling/disabling changed only routing: Deployment generation/UID, pod UID, and template hash stayed fixed. Suspension and auto-sleep could independently scale the workload to zero while maintenance routing remained public; resume restored replicas without clearing maintenance. Temporary resources were deleted and the pre-existing controller deployment was restored Ready. Full dated evidence and identifiers are in `docs/render-artifacts/maintenance-mode.md`.

## Implementation summary (2026-07-15)

- **CRD** (`lego/types/v1alpha1/app_types.go`): `MaintenanceModeSpec{Enabled, URI}` on `AppSpec.MaintenanceMode`.
- **Operator** (`lego/operator/internal/controller/app_controller.go`, `cmd/activator/main.go`): maintenance has public-routing precedence over suspension and auto-sleep but never overrides their replica policy. Same-namespace Apps route directly to the fixed responder; other namespaces receive an App-owned `ExternalName` alias. A bounded custom-page fetcher validates every redirect hop and resolved IP, rejects credentials/self/private destinations, caps time/redirects/body size, serves successful origin content as 503, passes through origin 4xx/5xx, and returns explicit 502 on fetch failure.
- **Backend** (`lego/backend/internal/apps/{service,deploy,rest,graphql,mcp,render}.go`): paid-web/type/self-URL rules and one atomic two-key Core patch are shared by REST, GraphQL, MCP, create, and Blueprint validate/apply/re-sync. REST requires both keys; Blueprint permits omitted `uri` and preserves maintenance state when the whole field is absent. A maintenance-only update opens no deploy. URI/toggle effects are emitted only after persistence, exactly once and in deterministic URI-then-enabled order.
- **Dashboard**: Settings → Maintenance Mode section (toggle + custom-page URL, confirm dialog on enable) + a destructive-variant banner on the service-detail header while enabled.
- **Docs**: `docs/render-artifacts/maintenance-mode.md` (capture + parity decisions); `docs/ADR018-render-parity.md` new row (explicitly distinguished from the "Maintenance runs" non-goal); `docs/ADR006-bex-api.md`/`docs/ADR007-restart-suspend-and-resume.md` lifecycle notes.
- **Incidental fixes required to make this shippable** (the activator was built at w1/m4 but never actually deployed until this milestone, so several latent gaps surfaced for the first time):
  - `config/default/kustomization.yaml` never included `../activator`; both it and `../staticserver` hardcoded a redundant `bex-` prefix that double-applied under kustomize's `namePrefix`, producing `bex-bex-static-server` — fixed by dropping the hardcoded prefix (matches the `manager`/`api` convention) and wiring `../activator` in.
  - `make deploy`'s image substitution was scoped to `config/manager/`, silently missing sibling binaries. A disposable `config/deploy` overlay now applies the shared image transform above `config/default` without rewriting the checked-in base during deploy/build-installer.
  - The activator's RBAC was a namespace-scoped `Role`/`RoleBinding`, but `findAppByHost` lists Apps across every tenant namespace — converted to `ClusterRole`/`ClusterRoleBinding` (matching the manager's own Apps grant).
  - A severe, unrelated bug was also hit live: the manager's cache sync deadlocked forever on a fresh deploy because the KeyValue controller's cluster-wide `Secret` watch had no matching RBAC. Worked around locally (a temporary `ClusterRole` grant, not committed) to complete this milestone's live verification, and filed in `docs/ADR028-security-review.md`'s follow-up register — independently fixed the same day by a concurrent session (`fa49fbbd fix(operator): scope secret cache to apps namespace`, w10/m1), landed via the `git pull --rebase` this milestone's `/ship` picked up.
  - **Filed, not fixed** (`docs/ADR028-security-review.md` follow-up register): the outbound-webhooks path (`lego/backend/internal/webhooks`) has the same tenant-URL SSRF gap this milestone closed for the activator, ported nowhere else.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker` round 6, 2026-07-14 — field-level spec-grep of Render's live OpenAPI (`maintenanceMode`: `{enabled: boolean, uri: string}` on `webServiceDetails` + POST/PATCH; `uri` = "the page to be served when maintenance mode is enabled", linking render.com/docs/maintenance-mode). Zero hits in `lego/`.
- **NOT the ledger's non-goal:** ADR018 marks "Maintenance runs" `—`, but that row is Render's `/maintenance` **managed-infra** surface (platform-scheduled runs). This field is the **tenant-facing toggle** — a different capability the row-level audit never inventoried. Recorded here so the anti-goal screen isn't misread.
- **Goal linkage:** Render parity (service contract) + GOAL #1's lifecycle verbs — the missing state between "serving" and "suspended" (offline-with-a-page, pods intact).
- **Expected outcome:** deliberate downtime stops looking like an outage; one fewer permanent allowlist entry for `w7/m30`'s conformance suite.
- **Why now:** verified-in-spec gap; the serving-plane seams it needs (Traefik middleware or the static-server default-page path, the activator's interstitial precedent) all exist; w1 is at two actionable milestones. Render parity task included — all-surface change.
