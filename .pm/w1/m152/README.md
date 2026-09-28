# w1 · m152 — Canceling a config-change deploy still ships the change

**Worker:** worker1 **Goal:** canceling an in-progress deploy leaves the service running its last successful release. That means the image **and** the configuration (environment variables, secret files, linked group values, start/health/pre-deploy commands, plan) that release ran with. A saved change whose deploy was canceled stays saved and is shown as not deployed until a later deploy ships it. No pod ever rolls without a deploy row saying so. **Status:** todo (unblocked 2026-09-15; see § Decisions).

## Triage (2026-09-15)

Triaged on `main` at `5523f684e`: the bug is still real and nothing has fixed it.

- A save rewrites the single mutable `<name>-env` Secret and bumps `restartedAt` (`secrets/service.go:713-726`).
- Cancel only annotates (`deploys/service.go:720`), and `settleCanceledRelease` re-dispatches the old image against the current spec (`app_controller.go:634`).
- Rollback sets only image and `restartedAt` (`deploys/service.go:843-845`).

## Decisions (2026-09-15)

All three questions were answered by the user ("act as you recommended"), so this milestone is unblocked and proceeds in task order.

1. **Rollback restores the target deploy's values; cancel does not.** A rollback restores the target release's environment values and start command into the saved state, which is Render's documented "matches the target deploy" (option a) — so t009's `GET …/env-vars/MESSAGE` reads the target's value after a rollback. A **cancel** keeps the newer saved value and shows it as not deployed (option b), because a canceled deploy shipped nothing and must not overwrite what the user saved.
2. **No fleet-wide roll: migrate lazily.** Existing services keep their current pod template until their next deploy, which is when they pick up their release snapshot. Nothing rolls on the operator upgrade itself. The cancel and rollback paths must therefore handle a release that predates snapshots by falling back to today's behavior, and t004's blast radius covers that mixed state.
3. **Retention: the live release always, plus the last 10 releases per App** (including one snapshot per linked env group), older ones garbage-collected. t001 first checks how deep a rollback the deploy list actually offers; if it is deeper than 10, retention matches that window instead.

## Design input (t001 groundwork, 2026-09-16)

A full sweep of what a release's configuration actually consists of, so the snapshot design is complete before any code moves.

**Nothing is per-release today.** The App CR's `Spec` is the shared mutable object every writer patches, and the closest thing to a release record — `store.Deploy` — carries only image, resolved image, generation and commit. `status.releaseFingerprint` is a one-way SHA-256: proof of identity, never a restorable value.

**The config a pod actually gets, and where it lives:**

| Source | Object | Scope |
| --- | --- | --- |
| Service env vars (bulk) | Secret `<name>-env` (`secrets/service.go:688`), backed by OpenBao | per-App, rewritten whole on every save |
| Service secret files | Secret `<name>-files` (`secrets/files.go:56`), one `/etc/secrets` projection | per-App, rewritten in place |
| Linked group vars / files | Secrets `<evg-id>-env` / `<evg-id>-files` (`envgroups/service.go:210-211`) | **shared across every linked App**, no owner ref |
| Native build env | Secret `<name>-native-env` (`app_controller.go:1296`) | per-App, and part of **artifact** identity |
| Literal env, start/build/pre-deploy commands, health-check path, tier, disk intent, `maxShutdownDelaySeconds` | inline in `AppSpec` | per-App, mutable in place |
| Two-phase "saved, not deployed" staging | `PendingEnvSecretAnnotation` / `PendingFilesSecretAnnotation` | consumed by _whichever_ later reconcile rebuilds the template |

**Why the canceled change still ships.** `Cancel` writes exactly one App-adjacent thing — the canceled-generation annotation (`deploys/service.go:781-787`); it never touches env, files, commands, tier or `restartedAt`. The operator then dispatches the prior **image** against the **live spec** (`settleCanceledRelease` → `dispatchRuntime`, `app_controller.go:645`), and the pod template is rebuilt from `app.Spec` on every pass — so the config the canceled deploy already wrote is what runs. `Rollback` is the same shape: image + `restartedAt` only, with the doc comment saying so outright (`deploys/service.go:848-849`).

**Env groups are a live reference, not a copy.** A service stores only the group's Secret _name_; kubelet resolves contents at pod creation. A later group edit fans out synchronously to every linked service (`envgroups/patch.go:294-318`). A snapshot must therefore capture each linked group's **resolved contents and their precedence order** (groups first, then the service's own — `app_controller.go:3904-3934`), not just the names release identity records today.

**The hard constraints the design must answer:**

1. **Name stability.** `<name>-env` / `<name>-files` / `<name>-native-env` are pure functions of the App name and are read by other subsystems (`rejectProtectedSecretRefs`, the native build cache, the CRD's own `EnvFromSecret`). A versioned replacement must keep those names resolvable or dual-write.
2. **Native builders bake env into the image.** For native/buildpack runtimes the build env is part of artifact identity, so "restore the old image" and "restore the old config" are not cleanly separable — the decision that rollback restores the target's values (§ Decisions 1) needs an explicit answer for that runtime.
3. **Two independent choke points.** Every App-spec write funnels through either `rollout.Tracker.Patch` or `deploys.Service.patchApp`. A snapshot mechanism honored by only one is silently bypassed by the other.
4. **Group Secrets carry no owner ref** (one group serves N Apps), so per-release copies of group-derived content cannot reuse the App-ownerRef GC pattern.
5. **Quota and duplication.** Env maps are already bounded (500 keys, 512 KiB) and OpenBao already versions every write — a per-release copy multiplies stored bytes against those ceilings and re-implements versioning that exists. Retention (§ Decisions 3) interacts directly with this.
6. **Disk is a singleton.** One PVC per App ever, with grow-only `sizeGB` deliberately outside the release fingerprint — only attach/detach/mount-path intent is release-scoped, never the volume's data.
7. **`restartedAt` is dual-classed** (both artifact and release identity) and is the only thing that actually rolls pods; a snapshot must not turn a restore into an unintended roll, nor skip one that is needed.

**Controls to re-run as regression evidence:** the `w6/m52` cancel-phase tests (`canceled_release_phase_test.go:40,83,113`), the `w6/m104` image-backed cancel pair (`:142,174`), the cancel/rollback suite (`deploys/cancel_rollback_test.go`, notably `TestRollbackRestoresPreviousLiveImage:332`, which currently asserts the image-only behavior this milestone changes), and the config-rollout suites in `secrets/`, `envgroups/` and `apps/`.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Snapshot each release's runtime configuration so a release can be restored exactly | 75m | — |
| t002 | Cancel settles to the last successful release's full runtime identity (image and config), never the current spec | 60m | t001 |
| t003 | Truth surfaces: the canceled change stays saved and reads "not deployed"; the Live row is what actually runs | 45m | t002 |
| t004 | Blast radius: every config source a `config_change` deploy carries, plus the m52/m104 controls | 45m | t002 |
| t009 | Rollback restores the target deploy's configuration (env vars, start command), and a dashboard rollback turns auto-deploy off | 60m | t001 |
| t010 | Live: canceling a health-gated rollout restores the probe-free template and settles Running | 30m | t003 |
| t005 | Render parity | 30m | t003, t004, t009, t010 |
| t006 | Simplify | 20m | t005 |
| t007 | Test coverage | 45m | t005 |
| t008 | Closeout | 10m | t007 |

## Definition of done

Each bullet can be repeated on a throwaway free web service (`bex-co/bex` `examples/hello-go`, docker; `GET /` answers `$MESSAGE`, else `OK`) whose first deploy is live and serving `OK`. Only states observed at filing time are listed:

- **The canceled change never reaches the running service.** `PUT /v1/services/<srv>/env-vars/MESSAGE {"value":"should-not-ship"}` opens a `config_change` deploy. While it is `build_in_progress`, `POST /v1/services/<srv>/deploys/<dep>/cancel`. For 5 minutes afterwards, `curl https://<svc>.onbex.co/` keeps returning `OK`, and `GET /v1/services/<srv>/instances` shows no instance created after the cancel. At filing time a new instance was created at **15:28:51Z**, 4 s after the cancel, and the service answered **`should-not-ship`** by 15:29:57.
- **The saved value stays saved, and the next deploy ships it.** After the cancel, `GET /v1/services/<srv>/env-vars/MESSAGE` still returns `should-not-ship`; a later manual deploy serves it. The first half held at filing time and must be kept; the second half is the target.
- **The deploy list tells the truth.** After the cancel, the row `GET /v1/services/<srv>/deploys` and `/services/<srv>/deploys` mark **Live** is the release whose image and config are actually running, and every pod rollout has a deploy row. At filing time the list read `Canceled · config_change` over `Live · First Deploy (324ab60)`, while the running pod carried the canceled config and no row or event recorded that rollout.
- **Canceling a health-gated rollout restores the probe-free template and settles Running.** On a throwaway free web service: set Health Check Path to a 404ing path, wait for the `config_change` deploy to sit `update_in_progress` with its new pod listening-but-unready, cancel it — within 2 minutes the phase reads `Running` with exactly one serving pod, no pod carries the canceled probe, and the saved path still reads as not deployed. At extension time (t010, 2026-09-17) the revert pod carried the probe, the phase sat `Deploying` 8+ minutes with zero in-flight work, and only the pre-cancel pod served.

## Evidence (probes run 2026-09-14, production, workspace `bex` / `tea-d98210cbbpdc73dcrkvg`)

Fixture: free web service `qa-20260914-cancel` (`srv-dak13va6m8ac739r61h0`). Its first deploy `dep-dak13va6m8ac739r61hg` (`create`, commit `324ab6003`) was live at 15:28:17Z. The service was deleted at 15:31:52Z (`DELETE` → `204`, then `GET` → `404`).

```text
15:28:30Z events env_vars_changed; deploy_started dep-dak157ggsm7s73f64ig0 trigger {"envUpdated":true,…}; build_started
15:28:31  PUT /v1/services/srv-dak13va6m8ac739r61h0/env-vars/MESSAGE {"value":"should-not-ship"} → 200
15:28:47  deploy dep-dak157ggsm7s73f64ig0 config_change build_in_progress
15:28:47  POST /v1/services/srv-dak13va6m8ac739r61h0/deploys/dep-dak157ggsm7s73f64ig0/cancel
          → 200 {"id":"dep-dak157ggsm7s73f64ig0","status":"canceled","trigger":"config_change",…}
15:28:47Z events build_ended {status:canceled}; deploy_ended {deployStatus:canceled}
15:28:48  service phase=Building   15:28:55 phase=Deploying   15:29:03 phase=Running
15:29:57  curl https://qa-20260914-cancel.onbex.co/ → 200 "should-not-ship"   (first sample of a watcher started after the cancel)
15:30:14  GET …/instances → [{"id":"srv-dak13va6m8ac739r61h0-4d20eddqbh38udq4ek2h","createdAt":"2026-09-14T15:28:51Z"}]
          GET …/deploys   → dep-dak157ggsm7s73f64ig0 config_change canceled | dep-dak13va6m8ac739r61hg create live
          GET …/env-vars/MESSAGE → {"key":"MESSAGE","value":"should-not-ship",…}
          events: nothing after 15:28:47
dashboard /services/<srv>/deploys header: "Running · Latest deploy · Canceled"; table: "Canceled … Config Change 1s | Live dep-dak13va6m8ac739r61hg 324ab60 … First Deploy"
```

- **The product's promise.** Cancel dialog: "The in-progress deploy will be stopped. The last successful deploy remains live." (`dashboard/src/features/services/locales/en.ts:3852`).
- **Render** (`render.com/docs/deploys`, fetched 2026-09-14): after a cancel, "your service continues running its most recent successful deploy (if any), with zero downtime".

No screenshots were taken; the transcripts above are the evidence.

## Root cause

- **The save mutates what the running release reads, before any deploy.** Saving a service variable rewrites the one `<name>-env` Secret in place and points `spec.envFromSecret` at it (`lego/backend/internal/secrets/service.go:714-717`). It then bumps `spec.restartedAt` (`:721-725`). There is no per-release copy of the old values.
- **Cancel restores only the image.** `Cancel` (`lego/backend/internal/deploys/service.go:683-747`) stamps `AnnotationCanceledReleaseGeneration` and deletes the build Job. The operator's `settleCanceledRelease` then calls `dispatchRuntime(ctx, app, app.Status.Image, port)` whenever a prior image exists (`lego/operator/internal/controller/app_controller.go:621-648`, `:633-634`). That is the previous **image** combined with the **current** spec.
- **Why that rolls pods.** `restartedAt`, `envFromSecret`, `envFromSecrets` and `filesFromSecrets` are all release identity (`lego/operator/internal/controller/release_identity.go`, `releaseIdentityInput`). The bumped spec therefore changes the pod template, and the new pod reads the already-rewritten Secret. The canceled config ships with the old image, and no deploy row records it.
- **Coverage gap.** `w6/done/m104` made cancel restore the previous **image** for image-backed services and verified "the running pod's image" matches; it never checked configuration. `w6/done/m52` settled first-deploy cancels to `Canceled`. Neither covers a config change riding on the canceled deploy.

## Blast radius

- **Every `config_change` source rides the same path**, with the spec already mutated before the deploy exists:
  - service env vars and secret files (`secrets/service.go`);
  - environment groups, whose shared `<evg>-env` / `<evg>-files` Secrets are rewritten in place for **every** linked service (`envgroups/service.go:653-657`), so canceling one service's deploy cannot restore the group's old values for it;
  - start command, health check path, pre-deploy command and plan, all in `releaseIdentityInput`.
- **Automatic supersede reaches the same path, not only a user's Cancel.** Verified in pass 20 (2026-09-14) on `qa-20260914-gen` (`srv-dak27726m8ac739r6260`, `examples/hello-go`, since deleted: `DELETE` 16:48:35Z → `204`, then `GET` → `404`; URL `404` at 16:48:44). Values are compared by SHA-256 prefix only.

  ```text
  16:44:47  dep-…f64j40 config_change live; curl serves len=44 sha8=5eac54f2 (the first generated MESSAGE)
  ~16:45:1x PUT …/env-vars [{MESSAGE, generateValue:true}, {KEEP, "k"}] → MESSAGE sha8=913b5b9c; opens dep-…f64j50
  ~16:45:1x PUT …/env-vars/MESSAGE {generateValue:true} → sha8=38c9b794; opens dep-…9r627g
  16:46:29  dep-…f64j50 → canceled (superseded, no user action); curl now serves sha8=38c9b794
            deploy list at that moment: 9r627g build_in_progress | f64j50 canceled | f64j40 live
  16:47:47  dep-…9r627g live   (78 s after its value was already being served)
  ```

  The superseded deploy settled through the same canceled branch and rolled pods from the current spec and Secret. The newest value therefore went out before its own deploy built, while the list still named an older deploy Live. t002 and t003 must cover supersede as well as a user's Cancel.

- **Must stay correct:**
  - cancel of a first deploy with no prior release (`Canceled`, `w6/m52`);
  - cancel of an image-backed deploy with a prior release (image restored, `w6/m104`);
  - supersede semantics, where a newer deploy stamps a newer generation (`deploys/service.go:714-715`).

## Adjacent classes

- **A code deploy canceled while a config change is also pending** must restore both.
- **`save_only` env-group writes** (no deploy) must not be rolled by an unrelated cancel.
- **Crash-restart or scale of an old-release pod** after a save already starts pods with the new Secret values, because the Secret is shared, not per release. t001 must close that as well, or record it as a separate, named limitation.

## Unverified (reasoned, not probed this run)

- **The first seconds after the cancel.** The watcher started after the cancel, so the service's response between 15:28:51 and 15:29:57 wasn't sampled.
- **Other surfaces and sources.** The cancel was sent over REST, not the dashboard dialog or GraphQL `cancelDeploy` (same core `Cancel`). Env-group, secret-file, start-command and plan changes followed by cancel were reasoned, not probed.
- **Render and env values.** Render's docs don't state whether a canceled deploy's env values reach running instances. Its rule is only that the most recent successful deploy keeps running.

## Dedupe

- `w2/done/m10` (cancel and rollback), `w6/done/m52` (first-deploy cancel phase) and `w6/done/m104` (image-backed cancel restores the image) are all closed and none covers configuration.
- `w1/m148` (Restart builds the branch head) and `w1/092` (env-group writes redeploy auto-deploy-off services) are adjacent but distinct.
- `.pm/DO_NOT_DO.md`: no conflict.
- Not fixed on `main` as of `324ab6003`.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 2026-09-14 pass 16, journey 3 (deploy cancel; no prior live-QA coverage of cancel).
- **Goal linkage:** pillar 1, the Render-compatible deploy lifecycle (`docs/ADR004-app-deployment.md`, `docs/ADR018-render-parity.md` deploy cancel row), and the service/deploy status invariants set by `w6/m52` and `w6/m104`.
- **Expected outcome:** cancel becomes a real abort. A bad environment change caught mid-build never reaches traffic, and the deploy list's Live row is always what is running.
- **Why now:** cancel is the emergency brake for a bad config push (a wrong `DATABASE_URL`, a broken feature flag). Today it reports success while shipping the change, and the history then shows no trace of the rollout.
- **Render parity:** included. The fix changes deploy/service semantics visible on REST, GraphQL, MCP and the dashboard deploys page and header.
