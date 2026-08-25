# w6 · m51 — Settings-page spec changes never create a deploy-history row; a failed one strands the service in stuck Building

**Worker:** worker6 **Goal:** every rollout that actually rebuilds/redeploys a service is visible in its Deploys tab and Events feed, and a service never gets stuck in an unrecoverable phase after a config edit **Status:** in progress — code complete and gated green; t003/t007 blocked on the post-ship live repro

## Tasks (in order)

| id | title | est | depends_on | status |
| --- | --- | --- | --- | --- |
| t001 | Enumerate and live-confirm every Settings verb that forces an untracked operator rebuild | 45m | — | — **DONE** |
| t002 | Route build-relevant Settings mutations through the same deploy-tracked path as Restart | 90m | t001 | — **DONE** |
| t003 | Verify stuck-phase recovery: a corrected Settings value must self-heal, not just retry | 45m | t002 | — **BLOCKED** (needs the fix deployed) |
| t008 | Env-group link/unlink also forces an untracked rebuild — bug isn't confined to apps.Service.SetXxx | 30m | t001 | — **DONE** |
| t004 | Render parity: REST/GraphQL/MCP/UI agree on deploy-history visibility for these edits | 30m | t002, t008 | — **DONE** |
| t005 | Simplify | 20m | t004 | — **DONE** |
| t006 | Test coverage | 30m | t005 | — **DONE** |
| t007 | Closeout | 10m | t006 | — **BLOCKED** (gated on t003) |

## Definition of done

- `curl`/GraphQL: editing a service's Start Command (`setStartCommand`) via the API creates a new row in `deploys` for that service, visible via `GET /v1/services/{id}/deploys` and the `deploys(serviceId:)` GraphQL query, the same way `triggerDeploy` does today.
- Dashboard: Settings → Start Command → Save (through its confirmation dialog) shows the resulting rollout in the service's Deploys tab and Events feed, with a real status (Building → Live or Building → Failed), not silently absent.
- Live-verified failure path: set an intentionally-broken Start Command (a binary flag the app rejects) on a `qa-`-prefixed test service. Confirm the resulting deploy row reaches a terminal `Failed`/`Canceled` status (not stuck `Building`) within the platform's normal build-timeout window, and that the service header's phase converges to a stable, accurate state — not stuck reporting `Building` indefinitely.
- Live-verified recovery path: after the failure above, correct the Start Command back to a valid value and Save. Confirm this alone (no separate "Manual Deploy"/"Restart" click required) produces a new tracked deploy that reaches `Live`, and the service header reflects it without manual intervention.
- The same DoD holds for every build-relevant Settings verb enumerated in t001 (not just Start Command) — or each excluded verb has a stated, verified reason it doesn't force a rebuild.
- The same DoD holds for every build-relevant App-CR-patch call site enumerated in t008 (env-group link/unlink and the rest of the 16-site blast radius), not just the `apps.Service.SetXxx` family — or each excluded site has a stated, verified reason it doesn't force a rebuild.

## Implementation (2026-08-24)

**The fix.** The operator already publishes an exhaustive, test-guarded answer to "does this spec change force a rebuild or redeploy": `appSpecIdentityClasses` in `release_identity.go`, which names every `AppSpec` field artifact / release / operational. That table moved into the CRD contract module as `appv1alpha1.AppSpecIdentityClasses` + `SpecRollsRelease(before, after)` (`lego/types/v1alpha1/app_identity.go`), so bex-api gates its deploy-history writes on the same policy the operator fingerprints with — one table, one guard test, no second copy to drift.

`lego/backend/internal/rollout` is the shared seam every App-spec writer outside the deploy verbs now patches through: snapshot the spec, apply the mutation, ask `SpecRollsRelease`, stamp the release generation, merge-patch, and open a `deploys` row with trigger `config_change`. Wired at `apps.patchFetched` (the chokepoint all `SetXxx` and disk verbs share), `envgroups` link/unlink/roll/group-value-change, and `secrets` env-var/secret-file/batch writes. Once the row exists, the existing reconciler drives it to a terminal status; no new lifecycle machinery was needed.

**Two corrections to this milestone's premise, both load-bearing:**

1. `manual-deploy-button.tsx`'s "any spec change … unconditionally re-enters the build path" is Restart-context-specific and **not literally true**. Roughly half the `SetXxx` family is operational (scale, rename, autoDeploy, IP allow-list, maintenance mode, autoscaling, custom domains). A blanket gate on `patchFetched` would have minted a phantom deploy for every rename. The fix is correct in both directions, and `TestOperationalVerbsOpenNoDeploy` is the guard.
2. The "stuck `Building`" phase was **unbounded only in appearance**. The operator caps a rollout at `progressDeadlineSeconds = 900s` and bex-api observes it with an 18-minute `DeployGateTimeout`; the 2-minute observation window was mid-rollout. What was genuinely broken was that the rollout was _unobservable_ — no row to inspect, cancel, or retry, and no terminal status surfaced anywhere. That is what the fix closes.

**Scope beyond the filed blast radius.** All 16 App-CR patch sites from t008 are classified (table in `done/t008.md`); five needed tracking, eleven are genuinely operational or already correct. A multi-field REST/MCP `PATCH` is coalesced into one deploy row rather than one per field (`rollout.Batch`), which the store's overlap handling would otherwise have turned into one live row plus N-1 canceled ones.

**Remaining.** The DoD's live repro against `dashboard.bex.co` cannot run until the fix is shipped and deployed — the live control plane is still the pre-fix binary. t003 records the deterministic equivalent (`TestConfigChangeFailsTerminallyThenSelfHeals` replays the exact journey: broken edit → terminal `update_failed`, correction alone → new row → `live`) and what to re-run post-deploy. t007 closes once that lands.

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

## Live evidence (2026-08-24, workspace `bex`, service `qa-20260824-envprec` / `srv-da6bi90bjjms73fvga3g`, env group `qa-20260824-envgroup` / `evg-da6bjl8bjjms73fvga60`)

**The same untracked-rebuild bug recurs via a third, structurally distinct trigger — env-group linking — confirming the pattern is not confined to `apps.Service.SetXxx`. Full detail and the 16-site blast-radius grep in t008.**

Repro: created `qa-20260824-envprec` with a direct env var `MYVAR=direct_value`, reached `Live` (`dep-da6bi90bjjms73fvga40`). Created env group `qa-20260824-envgroup` with `MYVAR=group_value` and linked it to the service at group-creation time. `service.phase` transitioned `Running`→`Building`→`Running` and `Revision` bumped `rev-1`→`rev-2`, but `deploys(serviceId:)` never gained a second row — stayed at the single original deploy throughout. Root cause: `envgroups/service.go:947-957`'s `linkFetched` (shared by `LinkService` and the Blueprint-apply `LinkEnvGroup`) does a direct `s.Client.Patch` on the App CR, same shape as the `apps.SetXxx` family, no `triggerDeploy` call. `detachFetched` (unlink, same file ~line 985) is presumed the same shape, not independently live-verified.

Incidental, correctly-not-filed observation from the same repro: env var precedence when the same key is set both directly and via a linked group — the direct value won at runtime (`curl` confirmed `direct_value`, not `group_value`), matching expected/Render precedence. The dashboard does show both the service's own vars and each linked group's keys side-by-side (a collision is spottable by comparing sections) but has no explicit conflict indicator — a borderline UX gap, not pursued further.

Filed as **t008** (new task, `depends_on: [t001]`) with the full 16-call-site grep for whoever scopes t002's fix — the DoD above now covers this call site too.

## Investigated in the same session and correctly rejected — not filed

- **"Restart service" performing a full rebuild instead of a lightweight process restart:** not a bug — deliberate, documented design (`manual-deploy-button.tsx:22-34`): "bex has no way to restart pods without a new build," matches Render's own dropdown placement per the same comment. No UI copy claims a lightweight restart.
- **Env var value echoed by the app process appearing unmasked in runtime logs:** not a bug — logs are raw application stdout/stderr; no platform (including Render) scans log output for known secret values and redacts them, since the platform can't reliably distinguish "this looks like a secret" from ordinary data, and not printing secrets to stdout is the application's own responsibility. The dashboard's Environment tab masks-by-default with an explicit Reveal action (confirmed correct, matches the set value exactly) — that's the only place masking is a meaningful guarantee. Live-confirmed via a deliberately crafted Start Command that echoed a distinctive test value (`qa-20260823-SECRETVALUE-xyz`) to the runtime log, which appeared raw, as expected.
- **Usage-page resource counts, move-to-environment sibling risk, blueprint YAML validation:** covered by earlier iterations this same run, not re-litigated here.
