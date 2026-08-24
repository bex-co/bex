# w6 · m51 — Settings-page spec changes never create a deploy-history row; a failed one strands the service in stuck Building

**Worker:** worker6 **Goal:** every rollout that actually rebuilds/redeploys a service is visible in its Deploys tab and Events feed, and a service never gets stuck in an unrecoverable phase after a config edit **Status:** todo

## Tasks (in order)

| id   | title                                                                                   | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Enumerate and live-confirm every Settings verb that forces an untracked operator rebuild | 45m | —          |
| t002 | Route build-relevant Settings mutations through the same deploy-tracked path as Restart  | 90m | t001       |
| t003 | Verify stuck-phase recovery: a corrected Settings value must self-heal, not just retry   | 45m | t002       |
| t004 | Render parity: REST/GraphQL/MCP/UI agree on deploy-history visibility for these edits    | 30m | t002       |
| t005 | Simplify                                                                                  | 20m | t004       |
| t006 | Test coverage                                                                             | 30m | t005       |
| t007 | Closeout                                                                                  | 10m | t006       |

## Definition of done

- `curl`/GraphQL: editing a service's Start Command (`setStartCommand`) via the API creates a new row in `deploys` for that service, visible via `GET /v1/services/{id}/deploys` and the `deploys(serviceId:)` GraphQL query, the same way `triggerDeploy` does today.
- Dashboard: Settings → Start Command → Save (through its confirmation dialog) shows the resulting rollout in the service's Deploys tab and Events feed, with a real status (Building → Live or Building → Failed), not silently absent.
- Live-verified failure path: set an intentionally-broken Start Command (a binary flag the app rejects) on a `qa-`-prefixed test service. Confirm the resulting deploy row reaches a terminal `Failed`/`Canceled` status (not stuck `Building`) within the platform's normal build-timeout window, and that the service header's phase converges to a stable, accurate state — not stuck reporting `Building` indefinitely.
- Live-verified recovery path: after the failure above, correct the Start Command back to a valid value and Save. Confirm this alone (no separate "Manual Deploy"/"Restart" click required) produces a new tracked deploy that reaches `Live`, and the service header reflects it without manual intervention.
- The same DoD holds for every build-relevant Settings verb enumerated in t001 (not just Start Command) — or each excluded verb has a stated, verified reason it doesn't force a rebuild.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co` hosting surfaces, 2026-08-23/24 (run from a `/loop 30m` session). Originally investigating "does a manual Restart action hold its zero-downtime claim" and "does a secret value leak unmasked into logs" — both came back non-issues (see below) — but reproducing the Restart journey surfaced this while setting up its test fixture.
- **Goal linkage:** ADR006 (bex-api Render parity) — the Deploys tab and Events feed are meant to be the complete, trustworthy history of what actually ran on a service; a class of real rollouts invisible to both undermines that guarantee and, worse, can leave a service stuck with no visible next step.
- **Expected outcome:** no config change that forces an operator rebuild can leave a service in an untracked, unrecoverable state; every such rollout is a first-class deploy a user can see, retry, or roll back like any other.
- **Why now:** this is a genuine reliability/observability gap (not cosmetic) discovered live, with a full reproduction and a clean root cause already in hand — cheap to fix now, expensive to rediscover after a real user hits it silently.

## Live evidence (2026-08-23/24, workspace `bex`, service `qa-20260823-restart` / `srv-da5tjrpvtlks73b81sn0`)

**Repro steps:**

1. Created a fresh `qa-20260823-restart` Go web service (`examples/hello-go`), first deploy `dep-da5tjrpvtlks73b81sng` reached Live normally — this deploy IS correctly tracked (created via the New Service flow, which calls the create/deploy path).
2. Settings → Start Command → edited to `./app --qa-simple-test` (an unrecognized flag) → confirmed through the "Change the start command...?" dialog. `setStartCommand` GraphQL mutation returned 200 and the field genuinely persisted (`service(id).startCommand` confirmed via a direct GraphQL query = `"./app --qa-simple-test"`).
3. **No new row appeared in `deploys(serviceId: ...)`** — queried directly via GraphQL immediately after, and again ~2 minutes later: still only the original 2 deploys (the first-deploy `dep-...sng`, both `live`/`deactivated` — no third entry).
4. The service **did** actually rebuild and redeploy with the new (broken) command — Events feed shows `Instance failed` ("A running instance stopped passing readiness checks.") and `Build and deploy settings changed`, and `service.phase` (GraphQL) read `Building` continuously for the full observation window.
5. **`service.phase` stayed `Building` for 2+ minutes** with zero corresponding deploy row to inspect, retry, or cancel from the Deploys tab.
6. Corrected the Start Command to a second, valid value (`sh -c "echo ... && ./app"`) → confirmed through the same dialog → persisted correctly (verified via GraphQL) → **`service.phase` remained `Building`**, unchanged, for another 2+ minutes. The valid correction did **not** self-heal the stuck phase.
7. Only clicking the header's **Manual Deploy → Restart service** (a `triggerDeploy`-routed action, confirmed by code) produced a new, real, tracked deploy row (`dep-da5tp09vtlks73b81sug`) that rebuilt from the latest branch commit and reached `Live` normally, clearing the stuck phase.

**Root cause:** `SetCommands` (`lego/backend/internal/apps/service.go:3137-3160`, and by the same shape `SetRootDir:3034`, `SetDockerfilePath:3060`, `SetPreDeployCommand:3116`, `SetSourceAndRegistryCredential:3189`) patches `App.Spec` directly via `s.patchFetched(...)` and returns — it never calls the `deploys` package's create-deploy verb. Per `dashboard/src/features/services/components/manual-deploy-button.tsx:22-32`'s own maintained comment: "bex has no way to restart pods without a new build — any spec change increments the generation and unconditionally re-enters the build path," and "Both 'Deploy' and 'Restart service' route through the same `triggerDeploy` mutation (w2/m30 consolidation) so every rollout — including a restart — opens a deploy-history row." **`w2/m30` (done 2026-07-14) deliberately fixed this exact gap for the Restart action specifically** ("dashboard restart → `triggerDeploy`") — but that fix was never extended to the Settings-page mutations, which still patch the spec directly and rely on the operator's own generation-bump reconciliation to rebuild, invisibly.

**Blast radius (identified by code shape, not each independently live-verified — t001 confirms):** `SetCommands` (build + start command), `SetRootDir`, `SetDockerfilePath`, `SetPreDeployCommand`, `SetSourceAndRegistryCredential` (repo/branch/image change) all patch build-relevant `App.Spec` fields the same way, with no `triggerDeploy` call. `SetPlan`, `SetIdleTTL`, `SetDisplayName`, `SetAutoDeploy`, `SetNotifyOnFail`, `SetIPAllowList`, `SetMaintenanceMode`, `SetHealthCheckPath`, `SetMaxShutdownDelay`, `SetSubdomainPolicy`, `SetRoutes`/`SetHeaders`/`SetPublishPath` (static-site only), `SetAutoscaling` are **not** assumed in scope — t001 must confirm which of these actually force a rebuild via the operator's reconcile logic (the "any spec change" comment may be Restart-context-specific, not literally true for every field) before deciding whether the fix is a global gate on `patchFetched` or an allowlist on the build-relevant verbs above.

**Adjacent classes:** REST `PATCH` equivalents of these same verbs (Render-parity REST body) and the MCP tool equivalents share the same `Service.SetXxx` call, so REST/GraphQL/MCP/UI all inherit whichever fix lands — this is the Render-parity task (t004), not a separate bug per surface.

## Investigated in the same session and correctly rejected — not filed

- **"Restart service" performing a full rebuild instead of a lightweight process restart:** not a bug — deliberate, documented design (`manual-deploy-button.tsx:22-34`): "bex has no way to restart pods without a new build," matches Render's own dropdown placement per the same comment. No UI copy claims a lightweight restart.
- **Env var value echoed by the app process appearing unmasked in runtime logs:** not a bug — logs are raw application stdout/stderr; no platform (including Render) scans log output for known secret values and redacts them, since the platform can't reliably distinguish "this looks like a secret" from ordinary data, and not printing secrets to stdout is the application's own responsibility. The dashboard's Environment tab masks-by-default with an explicit Reveal action (confirmed correct, matches the set value exactly) — that's the only place masking is a meaningful guarantee. Live-confirmed via a deliberately crafted Start Command that echoed a distinctive test value (`qa-20260823-SECRETVALUE-xyz`) to the runtime log, which appeared raw, as expected.
- **Usage-page resource counts, move-to-environment sibling risk, blueprint YAML validation:** covered by earlier iterations this same run, not re-litigated here.
