# w8 · m160 — A cancel that races the traffic switch: the canceled release never serves, or the deploy is live

**Worker:** worker8 **Goal:** a cancel has Render's two outcomes and no third: accepted, the canceled release never answers a request and the live instance is not replaced; too late, the deploy is live and the cancel answers 409 `already live`. **Status:** blocked (2026-11-12: t001, t002, t004 and t005 done and shipped (2026-11-12); t003, the one decider for the switch, needs a cluster; t006's envtest and t007's replay follow it)

## Tasks (in order)

| id   | title                                                                                            | est | depends_on       |
| ---- | ------------------------------------------------------------------------------------------------ | --- | ---------------- |
| t001 | A cancel of a release `status.activeRevision` names answers 409 `already live` and stamps nothing — **DONE**| 45m | —                |
| t002 | The operator wakes on the cancel stamp — **DONE**                                                 | 20m | —                |
| t003 | One decider for the switch: a stamped release is never promoted, or one found serving is kept    | 2h  | t001, t002       |
| t004 | Render parity — **DONE**                                                                          | 15m | t001, t002, t003 |
| t005 | Simplify — **DONE**                                                                               | 15m | t004             |
| t006 | Test coverage                                                                                    | 45m | t004             |
| t007 | Closeout (live replay on production)                                                             | 45m | t006             |

## Definition of done

The note's Acceptance, on production (below): ten cancels at the first `update_in_progress` read of a free Node service that starts in under 5 s each end either `canceled` with the new build never answering and the same instance throughout, or 409 `already live` with the deploy `live`; a cancel right after the URL first answers the new build is 409 and adds no instance; no `server_failed`/`server_available`; the unchanged cases hold.

## Source + Goal linkage

- **Source:** promoted from inbox note w8/265 (A deploy canceled around the moment its new instance turns ready is recorded `canceled` while its build serves production, and the live instance is replaced twice), filed by `/qa-find-bugs-cli` w8 loop sweep `2ba7`, 2026-11-11 UTC; promoted by `/loopx w8` on 2026-11-12 because the fix spans the backend, the operator and a cluster check (~4h, more than one task).
- **Goal linkage:** Render parity for Cancel deploy (ADR018 Cancel row; ADR008 § Render parity first). Render parity is a task because the Cancel row changes.
- **Expected outcome:** the verb never reports success while the canceled build serves, and a healthy service is never restarted twice by a cancel.
- **Why now:** major severity; four of four cancels of a fast-starting service lost the race on production.

## Evidence

Free native Node web services with a health check path and auto-deploy off. The build command writes a stamp file and the server answers `ok <instance> stamp=<build>`, so each response names the build and the instance. The public URL was polled every 1–1.5 s; every poll in every case answered 200. Times are UTC.

**1. Cancel 0.8 s after the new build first answered (new pin).** Build `one` live on `…p1i66`.

```text
15:18:58    bex deploys create <srv>             → dep-…7800, build "four"
15:20:23.9  app log, instance …8l9n5: LISTEN stamp=four
15:20:26.9  …p1i66 is retired (its SIGTERM arrives 15:21:26.9, after the 60 s drain)
15:20:27.1  curl /  → stamp=four                  (first answer from the new build)
15:20:27.5  GET …/deploys/dep-…7800 → update_in_progress
15:20:27.9  bex deploys cancel <srv> dep-…7800   → "Deploy dep-…7800 successfully canceled", exit 0; status canceled
15:20:28    ok p1i66 stamp=one
15:20:29    a third instance …0hfm9 is created
15:20:30 – 15:20:50   14 polls: ok 8l9n5 stamp=four
15:20:51    ok 0hfm9 stamp=one;  15:20:53  ok 8l9n5 stamp=four
15:20:55 →  83 polls: ok 0hfm9 stamp=one
```

Events: `deploy_ended` `canceled` 15:20:27Z, then `server_failed` (`"unhealthy": "The instance failed its health check"`) 15:20:51Z and `server_available` 15:20:52Z, for a service that answered 200 throughout.

**2. Cancel at the first `update_in_progress` read, 1.5 s before the switch (new pin).** Build `one` live on `…uea95`.

```text
15:09:22    event build_ended
15:09:23.6  app log, instance …038vu: LISTEN stamp=two
15:09:24.2  status update_in_progress (polled every second)
15:09:24.9  bex deploys cancel                   → "successfully canceled"; event deploy_ended canceled
15:09:26.4  …uea95 is retired (its SIGTERM arrives 15:10:26.4, after the 60 s drain)
15:09:29 – 15:09:49   14 polls: ok 038vu stamp=two
15:09:36    a third instance …p1i66 is created; LISTEN 15:09:42.5 stamp=one
15:09:50 →  117 polls: ok p1i66 stamp=one
```

`bex deploys list` afterwards: build two `canceled`, build one `live`. Nothing records that build two served.

**3. Controls on the new pin: a cancel well before readiness holds.**

- Start command waits 8 s before listening; cancel at the first `update_in_progress` read (15:14:42.6). 190 polls all answered build `one` from the same instance. The canceled instance's pod was removed about 6.5 s after the cancel.
- Start command waits 45 s; cancel 15 s into the rollout (14:54:20.5). Same result; the pod was removed about 15 s after the cancel.

**4. Old pin, cancel 6 s before the new process listened.** Cancel acknowledged 14:38:11. The canceled deploy's instance logged `LISTEN` at 14:38:16.9 and answered the polls at 14:38:25 and 14:38:32. The live instance was retired at 14:38:19.5, a third was created at 14:38:21 and served from 14:38:39. Events `server_failed` 14:38:34Z and `server_available` 14:38:37Z were written.

**5. Old pin, cancel after the switch.** The new build answered at 14:47:05 while the status had just turned `update_in_progress`; cancel acknowledged 14:47:05.9. Build `two` answered 60 consecutive polls until 14:48:39, then a third instance took over with build `one`. The production roll overlaps the end of this run, so its 95 s is an upper bound, not a measurement of the old pin alone.

**6. Control: a cancel during the build** takes effect at once and the live instance is untouched. A second cancel answers 409 `deploy "…" is already canceled`, a cancel of the live deploy 409 `is already live`, an unknown id 404.

## Expected

Render, [Deploys](https://render.com/docs/deploys):

- "your original instance continues to receive all incoming traffic while the new instance is spinning up."
- "Your service continues running its most recent successful deploy (if any), with zero downtime."
- "You can cancel an in-progress deploy in the Render Dashboard by going to your service's Deploys page".

ADR018's Cancel row states the bex contract: "canceling a deploy leaves the service on its last successful release … No pod rolls", citing Render for "cancellation without activating an unready release". w4/405 puts it as: "on Render a canceled deploy never served. The bex record must keep that meaning."

So on bex a cancel has two outcomes and no third:

- **Accepted:** the canceled release never answers a request, and the live instance is not replaced.
- **Too late:** the release already serves, so the deploy is live. Cancel answers 409 `already live` and the row closes `live`.

## Root cause

1. **`Cancel` accepts a release that already serves.** `deploys.Service.Cancel` refuses only a row whose `FinishedAt` is set (`lego/backend/internal/deploys/service.go:1011`). It never reads the App's status. The row closes `live` in a later backend pass: since w4/m320 t004 that is the first pass that sees `RolloutSettling` with `activeRevision` naming the release (`releaseIsActive`, `lego/backend/internal/store/reconciler.go:1587`, `:1960`). Until that pass runs the row is open and cancelable. Case 1 hit it 0.8 s after the first answer.
2. **Nothing makes the traffic switch and the cancel exclusive.** The Deployment controller moves traffic on its own when the new pod is Ready. The cancel is a stamp the operator reads on a later pass (`markReleaseCanceled`, `lego/backend/internal/store/release_cancel.go:96-127`; `settleCanceledRelease`, `lego/operator/internal/controller/app_controller.go:789`). A stamp that lands within a few seconds of readiness is read after the switch.
3. **The stamp does not wake the operator.** It is an annotation, so it bumps no generation, and the primary App watch admits only generation, deletion, finalizer and three named annotation or label edges (`generationOrDeletionPredicate`, `app_controller.go:149-168`, over `event_predicates.go:30-43`). The `app-config-status` controller watches the cancel annotation (`saved_configuration.go:53-80`) but never dispatches runtime work (`TestSavedConfigurationEventsDoNotDispatchRuntime`, `saved_configuration_test.go:561`). The settle waits for a pod or Deployment update or a requeue. In the new-pin runs the settle became visible 1.5 s (case 1, third instance created), about 11 s (case 2, third instance created), 6.5 s and 15 s (case 3, canceled pod removed) after the cancel.
4. **The settle rolls back a release that took traffic.** The canceled branch rewinds to the last successful release (`lego/operator/internal/controller/release_identity.go:470-486`) and re-dispatches its snapshot. The old pod is already retired, so that is a third instance and a second swap.

**What w4/m320 already does.** For a web image release held for HTTP port discovery it pins the Services to the serving revision until the operator promotes the candidate (t001, t003), which makes promotion and cancel one decision. A native-runtime service with its port known is not held, and its rollout is the plain Deployment switch above.

## Fix

Target: the two outcomes under Expected, decided in one place.

1. **Backend guard.** Inside `markReleaseCanceled`'s re-read, under its optimistic lock, if `status.activeRevision` names the deploy's generation, write no stamp; close the row `live` through the same CAS and answer 409 `deploy "…" is already live`. The operator's status write changes the resourceVersion, so a stamp racing it retries and sees it.
2. **One decider for the switch.** Either extend m320's revision pin to every web and private rollout, so that a candidate takes traffic only when the operator promotes it and a stamped release is never promoted; or, if the plain Deployment switch stays, make the operator keep a canceled release it finds already active and report it, so the backend corrects the row to `live`. Pick one in the milestone; the first gives Render's sequence exactly.
3. **Wake on the stamp.** Admit a change of `AnnotationCanceledReleaseGeneration` or `AnnotationSurvivingReleaseGeneration` in `generationOrDeletionPredicate.Update`, so the settle starts when the stamp lands.
4. ADR018's Cancel row: state both outcomes.

## Acceptance

- A free Node service that starts in under 5 s, canceled at the first `update_in_progress` read, ten times. Each time either the cancel answers 200, the next 120 one-second polls never show the new build's stamp and `bex services instances` shows the same instance id throughout; or it answers 409 `already live` and the deploy reads `live`. Never a `canceled` row whose build answered a request.
- A cancel sent right after the public URL first answers the new stamp: 409, the row is `live`, no third instance.
- A cancel 15 s into a 45 s start: the canceled instance's pod is removed within about 2 s.
- No `server_failed` or `server_available` in any of these.
- Unchanged: cancel during a build; the surviving older deploy of w8/m117; the configuration restore of w1/m152; cancel of a first deploy (w1/m160); the held image release of w4/m320.
- Tests: `Cancel` on an open row whose generation is the active revision answers the conflict and closes the row `live`; an update that changes only the cancel annotation passes the primary predicate; an envtest cancel issued while the candidate pod turns Ready ends with the serving pod's UID unchanged or the row `live`.

## Unverified

- Operator timing is inferred from pod creation and `SIGTERM` times; its logs were not read. Pod updates also enqueue the App, so what delayed a given settle beyond the missing wake-up is not known. A port-detection scan can hold one pass for up to 15 s (`portDetectTimeout`, `lego/operator/internal/controller/port_detect.go:61`; see w8/259).
- In cases 1 and 5 the canceled release was already serving when the operator settled, yet it went back to build `one`. Whether `activeRevision` named the canceled release at that moment was not read.
- The `server_failed` pair was written in cases 1, 4 and 5 and not in case 2. What decides it was not traced; the outage it records is sampled (see w8/263).
- Image-backed and multi-instance services were not run in this sweep.
- Render does not document a cancel that races the traffic switch. Its deploy status `live` is terminal for cancel.

## Dedupe

w4/m320 (blocked on its live replay; its code is in the new pin) is the image web candidate that answers before promotion, and its t003 leaves "cancel after promotion" out of scope. w4/405 (done) closes a superseded row `live` when its release served; this is the user-cancel side of the same window. w1/m152 (done) restores configuration on cancel. w6/m104 (done) made the stamp reach image-backed services. w8/m117 and w8/m119 (done) are overlapping deploys. w6/m128 (done) is the build events of a canceled deploy. None guards `Cancel` against a serving release or decides the switch and the cancel together for an ordinary rollout.

## Cleanup

Both services were deleted after the reads; `GET /v1/services/<id>` answers 404.

## Post-release probe — QA loop round 193 (2026-11-12, pin `9a5b95e96210`, which carries t001 and t002's `bf4f30330`)

- **t001 passes on production.** On the Free Go web service `qa-20261112-r193-go` (`srv-dbqqp5i8vjhs73do9flg`), `POST /v1/services/{id}/deploys/dep-dbqqp5i8vjhs73do9fm0/cancel` against the live deploy answered `409 {"error":"conflict: deploy \"dep-dbqqp5i8vjhs73do9fm0\" is already live","id":"conflict"}`.
- **Cancels during the build.**
  - **First:** `dep-dbqqrn7fn2rs739v0r90`, a dashboard "Clear build cache & deploy", canceled from the deploy page at 12:16:17.
    - Its rows read `canceled` on the deploy page, in the list and in `deploy_ended`/`build_ended` (`canceled`), and the log ended `==> Deploy canceled`.
    - It was followed by `server_failed {"unhealthy":"The instance failed its health check"}` at 12:16:28 and `server_available` at 12:16:32, on the unchanged instance (`…1rt1mkhp4aesgs366n0m`, created 12:13:54).
    - That was during the platform rollout of this pin (deploy job done 12:18:38), so it is not attributed to the cancel.
  - **Second:** `dep-dbqqtcrk98kc7399u81g`, REST `POST …/deploys {"clearCache":"clear"}` then `POST …/cancel` at 12:19:50, on the settled platform.
    - It answered `200 {"status":"canceled"}`, and no `server_failed` followed in 45 s.
    - 1 s polls of the URL answered 200 throughout (66/66, 12:18:54–12:20:43).
- The race t003 decides (a cancel at the first `update_in_progress`) was not exercised. The fixture was deleted (DELETE 204, then GET 404).

## Post-release probe — `/qa-find-bugs-cli` sweep 11ae (2026-11-12, pin `9a5b95e96210`)

The definition of done's case "a cancel right after the URL first answers the new build is 409 and adds no instance" still fails on this pin, as expected until t003. One run on the free Node web service `qa-20261112-11ae-web` (`srv-dbqqtobk98kc7399u84g`). Its server answers `ok <pod> stamp=<build>`. The URL was polled about every 0.9 s; the cancel was the released CLI's `bex deploys cancel <srv> <dep> --confirm -o json`. Times are UTC.

```text
12:50:59.5  bex deploys create → dep-dbqrccrk98kc7399u910
12:52:20    build log: ==> Build successful 🎉        (no build_ended event yet)
12:52:35.8  app log, new instance …qabm25rutpl4gd4dc1ma: stamp=1794487903
12:52:41.9  curl → ok …65c9c4f556-tr7tf stamp=1794487903   (first answer from the new build)
12:52:41.9  bex deploys cancel → "Deploy dep-dbqrccrk98kc7399u910 successfully canceled", exit 0 (12:52:42.4)
12:52:42    events: build_ended {"buildStatus":"canceled"} and deploy_ended canceled
12:52:43 – 12:52:58  the canceled build answers (…tr7tf stamp=1794487903)
12:52:49.2  app log, a third instance …is6idh5q1ptv1rmpf1q3: stamp=1794487090 (the old build)
12:52:57 →  ok …6dccd56ff7-c7pj4 stamp=1794487090 (alternating with …tr7tf until 12:52:59, then only c7pj4)
```

- The cancel was accepted 0.5 s after the canceled build first answered, and that build kept answering for 16 s. The old build came back on a new instance. This is the Evidence's case 1 again, so t001's 409 does not cover the moment the switch has happened but `status.activeRevision` has not moved.
- New detail: `build_ended` reads `canceled` for a build whose log says `==> Build successful 🎉`, 22 s earlier. That event was written only at the cancel. Check this in t003/t006: a build that finished should end `succeeded` whatever happens to the deploy afterwards.
- No `server_failed` this time.
