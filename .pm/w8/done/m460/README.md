# w8 · m460 — A static site's first deploy goes live when a publish setting is saved during its build, and an IP rule saved meanwhile takes effect

**Worker:** worker8 **Goal:** saving a static site's publish directory or root directory while its first deploy builds no longer hangs that deploy for ~21 minutes and fails it with a health-gate message. The deploy goes live on the settings it started with, and an IP rule saved in the meantime blocks within seconds, as Render's do. **Status:** done (the definition of done passed on production pin `4b2cebd0bd03`, 2026-11-19; follow-up w8/339)

## Tasks (in order)

| id   | title                                                                                                                            | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Reproduce the hang in a test and name the exact mechanism — **DONE**                                                                        | 45m | —          |
| t002 | A static site's first deploy settles on its own release when a release input is saved during the build — **DONE**                          | 45m | t001       |
| t003 | An IP rule saved while a static site's first deploy is unsettled or failed takes effect on the content it serves within seconds — **DONE** | 30m | t001       |
| t004 | Blast radius: other release inputs, service types, save surfaces and the owed release — **DONE**                                           | 30m | t002, t003 |
| t005 | Render parity — **DONE**                                                                                                                    | 20m | t004       |
| t006 | Simplify — **DONE**                                                                                                                         | 15m | t005       |
| t007 | Test coverage — **DONE**                                                                                                                    | 30m | t005       |
| t008 | Closeout — **DONE** | 15m | t007 |

## Definition of done

Verified on production after a roll that carries the change, with bex CLI v0.7.0 or later, in `bex-canary` (a Scale workspace):

1. Create two static sites:

   ```sh
   bex services create --type static_site --name <fresh> --repo https://github.com/octocat/Spoon-Knife --branch main --publish-directory out --build-command 'mkdir -p out out2 && echo m-out > out/marker.txt && echo m-out2 > out2/marker.txt' --confirm -o json
   ```

   While each first deploy reads `build_in_progress`:

   - on the first site, run `bex services update <id> --publish-directory out2 --confirm -o json`;
   - on the second, run `--root-directory sub` then `--root-directory ''`.

2. Each first deploy reaches `live` within the usual ~4 minutes, not `update_failed` after ~21. GraphQL `service { phase }` reads `Running`. The first site serves the publish directory its deploy started with (`/marker.txt` → `m-out`). A following `bex deploys create <id> --wait` publishes `out2` (`m-out2`). The record of what t002 chose (snapshot, or the saved value when the build had not started, w21/051) matches what is served.
3. On a third fresh static site, save `--ip-allow-list 'cidr=203.0.113.0/24,description=doc'` during the first build, together with `--publish-directory`. Within 30 s of the save, a request from outside that range gets `403 Forbidden`, while the deploy is in flight and after it settles.
4. Controls, unchanged:
   - a static first deploy with no mid-build save goes live;
   - a mid-build rename or IP-rule-only save goes live, and the rule answers 403;
   - a non-first deploy with a mid-build root-dir save goes live;
   - a native web service's first deploy with mid-build saves goes live.
5. Delete every fixture by ID and confirm 404s.

## Finding

**Source:** `/qa-find-bugs-cli` sweep `0bbe`, 2026-11-17 07:20–08:10 UTC, production `https://api.bex.co/v1/`, pin `21762a1df1fb`, bex CLI v0.7.0 (Render CLI v2.36.0), human device OAuth, workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`, Scale plan). **Severity:** major. A deploy hangs and is then reported failed while the site serves. A rule the owner saved to lock the site down is not enforced for at least 36 minutes. **Attribution:** operator (static-site first release); the exact mechanism is unverified (t001).

### Repro

1. Static site `qa-20261117-0bbe-st` (`srv-dbu00mgmlqgs73cq4hdg`, since deleted) was created at 07:20:58 with `--publish-directory .` and a build command writing `root.txt` and `out/marker.txt`.
2. During its first build (07:21–07:22:38), `bex services update` saved, in order:
   - publish directory `out`;
   - `''`, refused "publishPath must not be empty";
   - `../x`, `/etc`, then `out` again;
   - build command cleared, then restored;
   - auto-deploy off;
   - a build filter;
   - root directory `sub`, then `''`;
   - `--ip-allow-list 'cidr=203.0.113.0/24,description=qa-doc-range'`;
   - a rename.
3. Build log: `07:23:56 ==> Uploaded in 37s` and `==> Build successful 🎉`. The deploy `dep-dbu00mgmlqgs73cq4he0` then read `update_in_progress` from 07:24:06.
4. From about 07:29 the site served the create-time publish: `/` → 200 (the repo's `index.html`), `/root.txt` → `qa-root-0bbe`, `/marker.txt` → 404. Requests came from a client outside `203.0.113.0/24`. GraphQL read `{"phase":"Deploying","publishPath":"out","revision":"rev-1","undeployedChanges":false}`, and `deploy { stallReason }` was empty.
5. 07:42:07: the deploy closed `update_failed` with `failureReason: "the deploy did not become healthy within the health-gate window; check the service logs"`. Phase stayed `Deploying`. At about 07:44, `/` still answered 200 to the same client.
6. Recovery: `bex deploys create srv-dbu00mgmlqgs73cq4hdg --confirm` at 07:54:37 went `live` at 07:58:14, phase `Running`, revision `rev-14`. Only then did `/` and `/marker.txt` answer `403 Forbidden`.

### Bisect

Fresh static sites from the same repo, each with one change:

| Site | Mid-build save | First deploy |
| ---- | -------------- | ------------ |
| `ctl` (`srv-dbu09rbvsrns73ebrpag`) | none | `live` in ~3.5 min |
| `ip` (`srv-dbu09rgmlqgs73cq4hmg`) | IP rule only | `live`; then `403 Forbidden` |
| `nm` (`srv-dbu09rrvsrns73ebrpd0`) | rename only | `live` |
| `pp` (`srv-dbu0bsgmlqgs73cq4hpg`) | `--publish-directory out` → `out2` | `update_in_progress`, served `out` (create-time), `update_failed` at 08:06:10 (created 07:44:50) |
| `rd` (`srv-dbu0bsgmlqgs73cq4hs0`) | `--root-directory sub` then `''` (net unchanged) | `update_in_progress`, `update_failed` at 08:05:54 |
| `ctl`, second deploy | `--root-directory sub` then `''` | `live` |

A native web service's first deploy took mid-build saves of root directory, branch and source in sweep `0594` and went live. So the hang needs a static site, its first release, and a mid-build save of a publish input.

### Cause (verified, t001, 2026-11-17)

The early return is `resolveDeployImage`'s first guard, `if decision.sourcePending && app.Status.Image == "" { return "", ctrl.Result{}, true, nil }` (`lego/operator/internal/controller/app_controller.go:1038` at pin `21762a1df1fb` and at `7a1530a7f`). t001's envtest (`static_first_release_save_envtest_test.go`) reproduces it with one save, or with `rootDir` set and cleared, and fails on that code with the production state: phase `Deploying`, Ready reason `Publishing`, `activeRevision: rev-1`, `image: ""`, `observedGeneration: 0`.

Pass by pass (create stamps `release-generation: 1`; each save stamps `pending-source-generation` one past its generation):

1. **Build:** gen-1 runs in `Building`. `releaseSourceFixed` is false (`status.buildRun` is written only for a failed build), so `firstReleaseReadsSavedSource` holds and `sourceUpdatePending` is false. The release in flight is pinned (`releaseInFlight`, release_identity.go:540) and builds its adopted view. A save that does not net out coalesces its generation into `pendingReleaseGeneration` (2 after one publish-directory save; the root-directory set-and-clear nets back to 0).
2. **Publish:** phase `Deploying`. `releaseSourceFixed` is now true, so `sourceUpdatePending` is true, but the release is still in flight (`releaseGeneration` 1 > `successfulReleaseGeneration` 0) and stays pinned (release_identity.go:518).
3. **First publish done:** `reconcileStaticSite` records `activeRevision: rev-1` and requeues for the host handover (app_controller.go:5020-5027). A static site writes `status.image` only after the handover, at :5034.
4. **Every pass after:** the release is no longer in flight (1 == 1), so the decision is `{sourcePending}` (release_identity.go:521-525), and `resolveDeployImage` returns at :1038 because `status.image` is empty. `reconcileStaticSite` never reaches `convergeStaticRouting`, `PhaseRunning` or `observedGeneration`. The host stays on the activator, which proxies a served static site to the static-server (w4/636), behind the middleware set the first route was written with (`writeFirstRoute` writes the first route only once, app_controller.go:3460). That is why the site answered 200 and ignored the IP rule.
5. bex-api's gate then closes the row `update_failed` with the generic health-gate line, because the App reports no verdict.

A direct publish (no build command) halts at the same line one step earlier, on a save during its upload, and a served direct site halted there on every pass while any save waited, so its routing (IP rules, suspension) stopped following the App. A web service records `status.image` before it serves, so the same saves never stranded it (t001 step 3; its envtest settles `Running` on `rev-1`).

### Render

- Render's update-service API says a settings change is not deployed automatically: "Changes will **not** be deployed automatically" ([update-service](https://api-docs.render.com/reference/update-service), read 2026-11-17). An in-flight deploy finishes on what it started with.
- Inbound IP rules: "Your new rules take effect within a few seconds" and "Disallowed IPs are automatically blocked with a `403 Forbidden` response". Static sites are covered on Scale and Enterprise ([inbound-ip-rules](https://render.com/docs/inbound-ip-rules), read 2026-11-17).

### Dedupe

- **w4/515 and w8/235** (done): a static deploy never closed because the publish directory was missing. Here the directory exists, and the trigger is a mid-build save.
- **w32/028** (done): a missing branch on a no-build static site.
- **w32/m419** (done): IP rules over failed image deploys, web services.
- **w21/051** (done): a first release that has not started building reads the saved source.
- No open item matches (`rg` over `.pm` for static first-deploy hangs, `update_in_progress` with static, and mid-build publish saves). No newer operator commit touches static first releases since pin `21762a1df1fb`.

### Blast radius (t004, envtest, 2026-11-17)

Each row is a fresh static site whose first build is running when the change lands (`static_first_release_save_envtest_test.go`). "Save" is the save-only shape (CLI and REST `PATCH /v1/services`, the dashboard's Settings save, REST env-var writes: `pending-source-generation`). "Deploy" is the shape that stamps a release (a Blueprint sync, GraphQL or MCP env-var writes, deploy verbs). "Operational" stamps nothing.

| Change during the first build | Shape | Before the fix | After |
| --- | --- | --- | --- |
| publish directory | save | stuck `Deploying` | `live` on `out`; `undeployedChanges: saved`; the next deploy publishes `out2` |
| root directory set, then cleared | save | stuck `Deploying` | `live`; nothing owed |
| environment variable, env-group link, secret file, branch, build command | save | stuck `Deploying` (same branch: every release input) | `live` on `rev-1`; nothing builds until a deploy |
| IP rule + publish directory | save | stuck; rule unenforced until a manual deploy | rule on the activator's route within a pass, kept through publish and after `live` |
| build filter, auto-deploy, headers and routes, IP rule alone, rename | operational | `live` (control) | `live` |
| publish directory | deploy (Blueprint sync) | `live`, then the owed deploy republishes `out2` from gen-1's image (no rebuild) | unchanged |
| direct publish: a save during its first upload | save | stuck `Deploying`, nothing served | `live` |
| direct publish, served: a save, then an IP rule | save + operational | rule never enforced while the save waited | rule enforced, `Running`, nothing republished |
| web service: a root-directory save during its first build | save | `live`, but the settled release requeued every second while the save waited (`pendingReleaseGeneration`) | `live`, nothing owed |

Workspace and environment IP rule layers converge through the same `reconcileIPAllowListMiddleware` call, so they follow the service rule. Private services and background workers record their image before serving like web services; their first releases are unaffected (the operator's full suite covers them).

Not filed: an absolute (`/etc`) or `../` publish path is accepted on a built site. The extract step already runs the tenant's own image (`publish.go:614-630`), the publish pod mounts no service-account token, and the guard for the clone-only path sits in `publish.go:466-471`.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` sweep `0bbe` (2026-11-17), a `bex services update` round trip on a static site. Its other flags passed: publish directory set (empty refused), build command clear and restore, auto-deploy, build filters, root directory, IP rules (403 once live), rename. Refused as expected: previews (a non-goal), `--image`, `--health-check-path`, `--plan`.
- **Goal linkage:** ADR008 core hosting and Render parity first; ADR029 static sites; ADR018's inbound IP allowlist row (w32/m419).
- **Expected outcome:** editing a new static site's settings while it first builds is safe. The deploy goes live in minutes, the edit applies on the next deploy, and an IP rule protects the site within seconds.
- **Why now:** a first deploy is exactly when people adjust a new site's publish directory. Today that hides the site's real state behind a false health-gate failure for ~21 minutes. Meanwhile it ignores an access rule the owner believes is in force.
- **Render parity included:** the deploy status reaches REST, GraphQL, MCP and the dashboard, and IP rules are a Render-documented behavior.

## Gate (2026-11-17)

t001–t007 shipped together (`fix(operator): … (w8/m460 t001–t007)`). Only t008 remains: replaying the definition of done in `bex-canary` needs a production roll that carries that commit. No decision or credential is owed; the gate clears by itself.

- **When:** production rolled to pin `db48cde2bae9` at 12:08 UTC on 2026-11-17, and `deploy.yml` rolls the first push (or its hourly cron) once the pin is at least 6 hours old, so not before about 18:08 UTC.
- **Check:** `git log origin/main --grep 'pin platform images'` names a pin at or after the fix commit (`git merge-base --is-ancestor <fix> <pin>`). Then t008 runs the definition of done's five steps with the newest bex CLI release, records each command and answer here, deletes every fixture by ID, and closes the milestone.

## Unblocked (2026-11-19, `/loopx w8` worker)

- **Affected:** t008. **Source:** verified evidence, no user decision.
- **Finding:** the newest pin on `origin/main`, `19faf35f3` (`chore(deploy): pin platform images to 4b2cebd0bd03`, 2026-11-19 09:13Z), contains the fix: `git merge-base --is-ancestor bc06052a5 4b2cebd0bd03` holds. Its `deploy.yml` run 44490624602 finished green. The next roll waits for that pin to turn 6 hours old (about 15:13 UTC).
- **Status:** pending verification. A deployed operator is not proof that the definition of done holds on production.
- **Next:** t008 replays the definition of done's five steps as the QA user in `bex-canary` with bex CLI v0.7.0, records each command and answer here, deletes every fixture by ID, and closes the milestone.

## Production replay (t008, 2026-11-19)

Production pin `4b2cebd0bd03` (carries `bc06052a5`), bex CLI v0.7.0 (Render CLI v2.36.0), the QA user's human device login, `bex-canary` (`tea-daif693dqjvc73e7as3g`, Scale). Run nonce `d623`, 09:44–09:57 UTC. Every site was created with the definition of done's `bex services create --type static_site … --publish-directory out --build-command 'mkdir -p out out2 && …' --confirm -o json`, and every save ran `bex services update <id> … --confirm -o json` while the deploy row read `build_in_progress`.

**Steps 1–2.** Both first deploys went `live` in about 3–4.5 minutes, with phase `Running`:

- **A** (`qa-20261119-d623-pp`, `srv-dbvc9u7h83kc738vm1dg`). The build log read `==> Building from https://github.com/octocat/Spoon-Knife@d0dd1f6` at 09:44:27, and `--publish-directory out2` was saved at 09:44:29. The deploy went `live` at 09:48:52, 4m28s after create, on `rev-1`, and `/marker.txt` answered `m-out`. GraphQL read `publishPath: "out2"`, `undeployedChanges: true`, reason `saved`. That record (snapshot taken, the saved directory waits) matches what was served. `bex deploys create <id> --wait` then went `live` in 2m48s (`rev-3`), `/marker.txt` answered `m-out2`, and `undeployedChanges` read `false`.
- **B** (`qa-20261119-d623-rd`, `srv-dbvc9ufh83kc738vm1fg`). `--root-directory sub` was saved at 09:44:32 and `''` at 09:44:33. The deploy went `live` at 09:47:29 (3m04s) on `rev-1`, `/marker.txt` answered `m-out`, and there was one deploy row and no rebuild. GraphQL read `undeployedChanges: true`, reason `saved`, though nothing differs from what is served. That false notice is filed as [w8/339](../../339.md): `SetRootDir` bumps `spec.restartedAt`, and `staticSaveWaits` compares the full release identity.

**Step 3.** C (`qa-20261119-d623-ipp`, `srv-dbvc9v0nl4hs73cv7pmg`) saved `--ip-allow-list 'cidr=203.0.113.0/24,description=doc' --publish-directory out2` at 09:44:32, mid-build. The first probe, 1 s later, got `403 Forbidden` from outside the range, and every probe through `update_in_progress` did too. The deploy went `live` at 09:47:25 (2m57s), and `/` and `/marker.txt` still answered `403`.

**Step 4 (controls).**

- No mid-build save: D (`srv-dbvcc97h83kc738vm1lg`) went `live` (4m14s, `Running`, `m-out`, `undeployedChanges: false`).
- IP rule only, 20 s into the build: E (`srv-dbvcc9onl4hs73cv7pqg`) answered `403` at the first probe after the save, went `live` (2m41s), and answered `403` afterwards.
- Rename only, 20 s into the build: F (`srv-dbvcca0nl4hs73cv7psg`) went `live`, `Running`, `undeployedChanges: false`.
- A non-first deploy: D's `bex deploys create`, then a root-directory set-and-clear 15–20 s into its build, went `live` in 2m46s on `rev-2`, `Running`. It also reads `saved` ([w8/339](../../339.md)).
- A native web service: G (`srv-dbvcdcgnl4hs73cv7pv0`, Node, `render-examples/express-hello-world`, free plan) took a root-directory set-and-clear 15 s into its first build. It went `live` in 4m05s, `Running`, `/` answered `200`, and `undeployedChanges` read `false`.

**Surfaces.** For A, B and C, REST `GET /v1/services/<id>/deploys` and MCP `list_deploys` returned the same rows and statuses: A `[live, deactivated]`, B and C `[live]`. The REST service read's `publishPath` agreed with GraphQL. The dashboard was not driven, because this host has no browser; it reads the same GraphQL fields (t005).

**Step 5.** All seven fixtures were deleted by ID with `bex services delete <id> --confirm` (09:57:26–36, exit 0). At 09:57:46, REST detail and each site's URL answered `404` for all seven, and the workspace list no longer named them. The CLI session was logged out and the Kratos session revoked.
