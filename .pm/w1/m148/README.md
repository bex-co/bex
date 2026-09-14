# w1 · m148 — "Restart service" silently deploys the branch head instead of the running commit, and a specific-commit deploy is not honored

**Worker:** worker1 **Goal:** a restart keeps the commit and configuration that are already running, on every restart surface. A deploy pinned to a specific commit stays pinned and cannot be undone by a later restart or push. The dashboard can make that pin, which today only the API can. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                     | est | depends_on       |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Restart keeps the running release on every surface (REST `…/restart`, GraphQL `restartServer`, MCP `restart_service`, dashboard header + row) | 60m | —                |
| t002 | Honor specific-commit deploys: dashboard "Deploy a specific commit", and pinning turns auto-deploy off (Render's contract)                  | 60m | —                |
| t003 | Restart copy and confirmation tell the truth, and the stale `BuildCommit` comment is corrected                                              | 20m | t001             |
| t004 | Render parity                                                                                                                             | 30m | t001, t002, t003 |
| t005 | Simplify                                                                                                                                  | 20m | t004             |
| t006 | Test coverage                                                                                                                             | 45m | t004             |
| t007 | Closeout                                                                                                                                  | 10m | t006             |

## Definition of done

Each bullet can be repeated on a throwaway free web service built from `bex-co/bex` `examples/hello-go`, pinned to an older commit whose `examples/hello-go` matches the branch head. Only bullets that were run at filing time are listed:

- **Restart keeps the running commit.** Pin with `triggerDeploy(serviceId, commitId: <older SHA C>)` and wait for `live` on C. Then choose **Manual Deploy → Restart service** in the service header. The resulting deploy's `commitId` must equal **C**. At filing time it resolved to the branch head **H** (`dd5082002`) while C (`f3284af44`) was running.
- **A specific-commit deploy lands on that commit (control).** `triggerDeploy(commitId: C)` produces a deploy whose `commitId` is C and which reaches `live`. This held at filing time.
- **Pinning turns auto-deploy off.** After that deploy, `service { autoDeploy }` reads `false`, matching Render ("This disables automatic deploys for the service"). At filing time it read **`true`**, so the next push would silently replace the pin.
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
