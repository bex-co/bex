# w1 · m148 — "Restart service" silently deploys the branch head instead of the running commit, and a specific-commit deploy is not honored

**Worker:** worker1 **Goal:** a restart keeps the commit and configuration that are already running, on every restart surface. A deploy pinned to a specific commit stays pinned and cannot be undone by a later restart or push. The dashboard can make that pin, which today only the API can. **Status:** done (2026-09-15). Every task is complete, and the definition of done passed live on production (images pinned to `c4212ec71`) on every restart surface.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Restart keeps the running release on every surface (REST `…/restart`, GraphQL `restartServer`, MCP `restart_service`, dashboard header + row) — **DONE** | 60m | — |
| t002 | Honor specific-commit deploys: dashboard "Deploy a specific commit", and pinning turns auto-deploy off (Render's contract) — **DONE** | 60m | — |
| t003 | Restart copy and confirmation tell the truth, and the stale `BuildCommit` comment is corrected — **DONE** | 20m | t001 |
| t004 | Render parity — **DONE** | 30m | t001, t002, t003 |
| t005 | Simplify — **DONE** | 20m | t004 |
| t006 | Test coverage — **DONE** | 45m | t004 |
| t007 | Closeout — **DONE** | 10m | t006 |

## Definition of done

Each bullet can be repeated on a throwaway free web service built from `bex-co/bex` `examples/hello-go`, pinned to an older commit whose `examples/hello-go` matches the branch head. Only bullets that were run at filing time are listed:

- **Restart keeps the running commit.** Pin with `triggerDeploy(serviceId, commitId: <older SHA C>)` and wait for `live` on C. Then choose **Manual Deploy → Restart service** in the service header. The resulting deploy's `commitId` must equal **C**. At filing time it resolved to the branch head **H** (`dd5082002`) while C (`f3284af44`) was running.
- **A specific-commit deploy lands on that commit (control).** `triggerDeploy(commitId: C)` produces a deploy whose `commitId` is C and which reaches `live`. This held at filing time.
- **Pinning from the dashboard turns auto-deploy off.** After **Manual Deploy → Deploy a specific commit** with C, `service { autoDeploy }` reads `false`, matching Render's dashboard ("This disables automatic deploys for the service"). At filing time no such control existed, and the API pin left it **`true`**. _Corrected 2026-09-14 (t002):_ this bullet originally required every `commitId` trigger to turn auto-deploy off. Render's API says the opposite — `POST /services/{serviceId}/deploys` `commitId`: "Note that deploying a specific commit with this endpoint does not disable autodeploys for the service" (pinned `lego/backend/internal/api/openapi/render-public-api-1.json`) — so the API trigger keeps auto-deploy unchanged and the dashboard dialog turns it off.
- **The dashboard can make the pin.** **Manual Deploy** offers **Deploy a specific commit** (a SHA input and/or recent commits). At filing time the menu offered only `Deploy latest commit · Clear build cache & deploy · Restart service`.

## Evidence (probes run 2026-09-14, production, workspace `bex` / `tea-d98210cbbpdc73dcrkvg`)

Fixture: free web service `qa-20260914-sc` (`srv-daju01q6m8ac739r5qr0`, `bex-co/bex`, root `examples/hello-go`, docker), created 11:52:44Z and deleted inside the run. After deletion its URL returns `404`. Branch head H = `dd508200284bee128caa975aa9af47a39bcb2223`. Pinned commit C = `f3284af44e2f00fbfe2b2f10f04ae5243e1e2bdf`: the last change to `examples/hello-go` is `3c8faf888` (2026-08-02), and `git diff --quiet C H -- examples/hello-go` passes, so both commits build identical code.

1. The first deploy `dep-daju01q6m8ac739r5qrg` (`create`) reached `live` on H.
2. **Pin.** `mutation { triggerDeploy(serviceId:"srv-daju01q6m8ac739r5qr0", commitId:"f3284af44e2f…") }` → `dep-daju1j8gsm7s73f64bk0` (`trigger: api`), `queued` → `build_in_progress` → **`live` at 11:57:19, `commitId: f3284af44…`**. Then `service { autoDeploy }` → **`true`**.
3. **Restart.** Header **Manual Deploy** offered exactly `["Deploy latest commit", "Clear build cache & deploy", "Restart service"]`. Clicking **Restart service** showed no confirmation and sent:

   ```text
   TriggerDeploy {"serviceId":"srv-daju01q6m8ac739r5qr0"}
   → {"data":{"triggerDeploy":{"id":"dep-daju2j8gsm7s73f64bo0","status":"created","trigger":"api"}}}
   ```

   The new deploy resolved to **`commitId: dd508200284bee128caa975aa9af47a39bcb2223`** (H), not C, and was `build_in_progress` on H when the fixture was deleted.

4. **Render's contract**, from `render.com/docs/deploys` (fetched 2026-09-14): a restart "creates a completely new instance of your service" and "the new instance always uses the exact same Git commit and configuration as the running instance at the time of the restart". Separately, "Deploy a specific commit … Specify a commit by its SHA, or by selecting it from a list of recent commits. This disables automatic deploys for the service."

No screenshots were taken; the transcripts above are the evidence.

## Root cause

- **Each restart surface goes to the branch head a different way:**
  - REST `POST /v1/services/{id}/restart` → `apps.Service.Restart` (`lego/backend/internal/apps/rest.go:739`), which sets `a.Spec.BuildCommit = ""` with the comment "clear any commitId pin so the restart uses Branch HEAD" (`lego/backend/internal/apps/service.go:2935-2941`).
  - GraphQL `restartServer` → `s.Trigger(ctx, serviceId, TriggerParams{})` (`lego/backend/internal/deploys/graphql.go:215-221`). With no `commitId`, `Trigger` resolves the branch and pins the resolved SHA (`deploys/service.go:552-561`), which is H, not the running commit.
  - The dashboard header's **Restart service** calls `trigger(service.id)`, identical to **Deploy latest commit** (`dashboard/src/features/services/components/manual-deploy-button.tsx:96-115`).
  - The Overview/list row **Restart** routes through `triggerDeploy` as well (`service-row-actions.tsx:34-51`).
- **The copy promises something else.** The row's confirmation says "The service's pods roll with no downtime. In-flight requests finish before old instances are replaced." (`dashboard/src/features/services/locales/en.ts:311-313`), which reads as a restart of the same release. The header path has no confirmation at all.
- **The planned design never shipped.** `w2/done/m30` t004 and t006 planned to route restart through `triggerDeploy(deployMode: "deploy_only")`, and flagged that the confirm copy had to say honestly whether it rebuilds. But `deploy_only` is refused for repo-backed services (`deploys/service.go:440-446`). What shipped sends no mode at all, so restart became "deploy latest".
- **Why every restart rebuilds.** `restartedAt` is part of the release identity (`lego/operator/internal/controller/release_identity.go:70,121`). That alone is not the bug (Render also makes a new instance). The bug is that the rebuild picks up **new code**.
- **Pinning.** `Trigger` with `commitId` sets `spec.BuildCommit` (`deploys/service.go:559-561`) but never touches `autoDeploy`. The dashboard offers no control for `commitId` (`manual-deploy-button.tsx`).
- **A stale comment.** `deploys/service.go:401-403` claims "the field is explicitly reset to "" on every trigger without a commitId so Branch HEAD is always the default". In fact every trigger pins the resolved SHA (`:552-561`).

## Blast radius

- **Five restart entry points:** REST `…/restart`, GraphQL `restartServer`, MCP `restart_service` (`apps/mcp.go:704`, handler not read this run), and the dashboard's header and row Restart. One target behavior has to cover all five.
- **Correct today and must stay correct:**
  - an ordinary **Deploy latest commit** (should keep resolving H);
  - a push-webhook deploy, which pins the pushed SHA (`apps/service.go:2891`);
  - an env-change `config_change` rebuild, which leaves `BuildCommit` untouched and therefore rebuilds the running commit;
  - Rollback.
- **Image-backed services:** restart must keep the running image, not re-resolve a moving tag. Unverified, but the same principle applies.

## Adjacent classes

- **Staged env changes.** Render's restart does **not** pick up environment changes saved but not deployed. bex has a staged (`save_only`) save mode, and a restart must not apply staged configuration either, unless t004 records a deliberate divergence.
- **Rebuild vs reuse.** A restart may rebuild the same commit, or reuse the running image. Reusing is cheaper and closer to Render, but `restartedAt` is release identity today. t001 decides and records the choice.
- **Restart while a newer deploy is in flight.** The restart targets the **live** release, not the in-flight one.

## Unverified (reasoned, not probed this run)

- The Overview/list row **Restart**, REST `POST …/restart`, GraphQL `restartServer` and MCP `restart_service` were not driven live. Their branch-head behavior is read from code: the row goes through `triggerDeploy`, and `restartServer` calls `Trigger` with empty params.
- Whether a pushed commit really overrides a manual pin while `autoDeploy` is `true` (no push was made to the fixture's branch).
- Whether a restart applies staged (`save_only`) env changes today.

## Implementation (2026-09-14)

**Mechanism (t001): option (a), rebuild the live commit.** `deploys.Service.Restart` (`lego/backend/internal/deploys/service.go`) authorizes like a parameter-free trigger (`can_operate`), checks billing, and for a repo-backed service reads the newest `live` deploy and triggers with its commit (falling back to `spec.buildCommit` when the live row's commit was never resolved). An image-backed service redeploys its configured image. No live deploy → **409** `service "…" has no live deploy to restart; deploy it first`. Option (b), reusing the live image without a build, was not contained: `restartedAt` is release identity (ADR004), and a restart annotation outside it would be a new operator contract. Every surface runs the one verb:

- GraphQL `restartServer` → `Restart`;
- REST `POST /v1/services/{id}/restart` and MCP `restart_service` → `apps.Service.Restart` → the `RestartDeploy` hook wired in `internal/api/server.go` (only when a deploy store exists; a store-less bex-api keeps the CR-only restart, and `restartServer` is unavailable there as before). `apps.Service.Restart` no longer clears `spec.buildCommit`;
- dashboard header **Restart service** (`useTriggerDeploy().restart`) and services-list row **Restart** (`useServiceLifecycle`) → `RestartServer`.

A repo-backed **cron job** also restarts on its live commit: the `commitId` refusal for cron jobs guards caller input, and the restart's commit is exempt through an unexported `TriggerParams` field no surface can set.

**Specific commit (t002).** Backend unchanged, per Render's API (see the corrected DoD bullet). Dashboard **Manual Deploy → Deploy a specific commit** (repo-backed, non-cron — Render: "Not supported for cron jobs") opens a dialog with a SHA input (7–40 hex characters, validated inline), says it turns auto-deploy off, triggers with `commitId`, then calls `setAutoDeploy(false)` when auto-deploy is not already off. A server refusal is relayed through `mutationErrorMessage` in the toast. Declined: a recent-commits picker (no cheap commit-list read exists; SHA-only), and a "turned off because of a pin" marker in Settings (Render shows none; the auto-deploy toggle already reads off).

**Copy (t003).** Both restart entry points share `services.confirmRestartTitle/Body`: "{name} restarts on the commit or image it is running now. Commits pushed since are not deployed. New instances replace the old ones once they are healthy." The header Restart gained the confirmation. The `Trigger` doc comment now describes the real pinning, and MCP `restart_service`'s description no longer promises a pods-only roll.

**Adjacent rules (t004 step 3).**

- **Auto-deploy after a pin** stays off until the user turns it back on (Render). A later **Deploy latest commit** does not re-enable it.
- **Staged configuration — divergence kept.** Render's restart does not apply environment changes saved but not deployed. A bex restart is a new release built from the running commit, and it starts with the service's current saved environment, the same as any deploy. Matching Render would require snapshotting environment per release, which bex does not do; recorded in ADR018's Restart row.

**Simplify (t005).** Three review passes (reuse, quality, efficiency). Applied: the SHA input uses the shared `TextField`; the typed SHA is cleared whenever the dialog closes (it used to survive a cancel or refusal); redundant `setDialog(null)` calls removed (`ConfirmDialog` closes on confirm); one `REFETCH_DEPLOYS` options constant for both mutations; history-narrating comments trimmed; `*deploys.Service.Restart` pinned to `can_operate` in the role ladder; `deploys.Restart` excused in the events vocabulary like `Trigger` (its deploy row is the event). Declined: moving the cron `commitId` refusal from `validateTrigger` into `Trigger` to drop the unexported `restart` field — the deploy hook forwards `?ref=` as `CommitID` through the same validator, so the refusal must stay shared; returning the patched App from the hook to save `apps.Restart`'s second fetch (one extra read on a user-initiated verb); a shared helper for `trigger`/`restart`'s try-toast shape (two call sites); reusing `LatestDeployCommit` (it returns the newest commit of any deploy, including building or failed ones).

**Tests (t006).** Backend: `deploys/restart_test.go` (live commit not branch head with a resolver returning H, live release not a newer building/failed deploy, 409 with no live deploy and no row opened, `buildCommit` fallback, image-backed keeps its image, cron job, GraphQL `restartServer`) and `apps/restart_test.go` (REST restart runs the wired deploy verb and relays its refusal; unwired restart keeps the pin). `TestGraphQLRestartServerKeepsTheLiveCommit` and `TestRestartKeepsTheBuildCommitPin` were run against pre-fix `HEAD` (`a2fb27559`) in a scratch worktree and failed with exactly the bug (`restartServer commitId = dd5082002…, want f3284af44…`; `spec.buildCommit = "", want the pin kept`); the other deploys tests call the new verb and cannot compile pre-fix. Dashboard: `manual-deploy-button.test.tsx` (confirmation then `restart`, cancel restarts nothing, specific-commit item hidden for image-backed and cron, SHA trimmed and sent, auto-deploy turned off only after a successful deploy and only when on, malformed SHA keeps confirm disabled), `use-trigger-deploy.test.ts` (restart uses `RestartServer`, refusal resolves null), `use-service-lifecycle.test.ts` (row restart fires `RestartServer`).

## Live verification (2026-09-15, production pinned to `c4212ec71`)

- **Fixture.** `qa-20260915-m148` (`srv-dakec9h5v75s738uf830`), a free web service from `examples/hello-go` (docker), created with `autoDeploy: no`.
  - Its first deploy `dep-dakec9h5v75s738uf83g` went live at 06:47:31Z on the branch head H (`c4212ec71`).
  - C = `3ca4dd2485a058029a8c88cc1791db6378e6e2b5`, older than H; `git diff --quiet C H -- examples/hello-go` passes.
- **Control (pin), before the rollout, 06:49:38Z.** GraphQL `triggerDeploy(serviceId, commitId: C)` produced `dep-dakel0fnonls738jbna0`, live on C at 06:51:01Z.
- **After the rollout.**

| Surface | Action | Resulting deploy | Commit |
| --- | --- | --- | --- |
| Dashboard header (GraphQL `restartServer`) | Manual Deploy → Restart service → "Restart qa-20260915-m148?" → Restart, 07:33Z | `dep-dakfa0pvi8js739uilgg` live 07:34:52Z | C `3ca4dd248` |
| REST | `POST /v1/services/{id}/restart`, 08:02:52Z | `dep-dakfnb9vi8js739uilng` live 08:04:26Z | C `3ca4dd248` |
| MCP | `tools/call restart_service {serviceId}`, 08:04:56Z | `dep-dakfoabidljc7398tnl0` live 08:06:26Z | C `3ca4dd248` |
| Dashboard | Manual Deploy → Deploy a specific commit → C → Deploy commit, with `autoDeploy` set to `yes` first | `dep-dakfapjidljc7398tncg` live 07:37:26Z | C `3ca4dd248`; `autoDeploy` afterwards `no` |

- **Copy.**
  - The Manual Deploy menu offers Deploy latest commit · Deploy a specific commit · Clear build cache & deploy · Restart service.
  - The restart confirmation reads "qa-20260915-m148 restarts on the commit or image it is running now. Commits pushed since are not deployed. New instances replace the old ones once they are healthy."
  - The specific-commit dialog reads "Deploys the commit you enter and turns off auto-deploy, so later pushes don't replace it. You can turn auto-deploy back on in Settings.", with Deploy commit disabled until the SHA is valid.
- **Every DoD bullet passed.**
  - A restart keeps the running commit C on every surface.
  - A specific-commit deploy lands on C.
  - Pinning from the dashboard turns auto-deploy off, while the API pin leaves it unchanged, per Render's API.
  - The dashboard can make the pin.

## Dedupe

- `w2/done/m30` built `commitId` (`App.spec.BuildCommit`) and consolidated restart into `triggerDeploy`. Its restart shape (`deploy_only`) was never shippable for repo-backed services, and its "honest rebuild copy" criterion never landed. This finishes that intent and does not reopen a closed decision.
- `docs/ADR018-render-parity.md` marks **Restart** (line 75) and **Trigger a deploy** (line 96) ✅ on all four surfaces, without the commit semantics.
- `w3/done/m46/done/t003` added **Clear build cache & deploy** to the same menu, and does not cover restart or a specific commit.
- No open or done board item names restart picking up new code.
- `.pm/DO_NOT_DO.md`: no conflict.
- Not fixed on `main` as of `dd5082002`.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 2026-09-14 pass 9, journey 3 (Deploys: manual deploy, specific commit, restart). The fixture, probes and deletion are above.
- **Goal linkage:** pillar 1, Render-compatible deploy semantics on REST, GraphQL, MCP and the dashboard (`docs/ADR004-app-deployment.md`, `docs/ADR018-render-parity.md`).
- **Expected outcome:** **Restart** is a safe action again, meaning it never ships unreviewed commits. A pin to a known-good commit (the common "roll back to that SHA" move) survives restarts and pushes, and users can make that pin from the dashboard.
- **Why now:** restart is the reflex action when a service misbehaves, and today it deploys whatever is on the branch. That is the worst moment to ship new code, and it quietly undoes a manual pin. Every repo-backed service is affected.
- **Render parity:** included. The fix changes restart and deploy semantics on REST, GraphQL and MCP, adds a dashboard control, and t004 updates the ADR018 rows.
