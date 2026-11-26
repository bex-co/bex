# w4 · m671 — bex checks a running image service's image and tells the workspace when it disappears, as Render does

**Worker:** worker4 **Goal:** when the image a running image-backed service serves is deleted from its registry, or its credential stops working, the workspace hears about it (event, webhook, email) while the instances keep serving, before a restart, a scale-up or a rollback fails on it. **Status:** blocked (t001–t003, t005–t007 done; t004 dev-4 half done; production half waits for the deploy)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Design note: what is checked, how often, and when a failure counts — **DONE** | 30m | w4/m669/t001 |
| [t002](done/t002.md) | The operator checks each running image service's served image and records `ImageAvailable` with its cause — **DONE** | 1h45m | t001 |
| [t003](done/t003.md) | bex-api records one `image_pull_failed` per outage with no deploy, says the instances still serve, and sends one email — **DONE** | 1h | t002 |
| [t004](t004.md) | dev-4 and production: an image that expires from `ttl.sh` raises the event and the email, and a recovery clears it | 45m | t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 25m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | t005, t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

Total: **8 tasks, about 5h20m**.

## Definition of done

- Every running (not suspended) image-backed service's served image is checked on t001's interval with the service's own credential, by a manifest `HEAD` that downloads no layer.
- An image that is gone (not found) or whose credential is refused, on t001's consecutive-check rule, sets `ImageAvailable=False` on the App with Render's cause. A rate-limited or unreachable registry never does.
- That transition records exactly one `image_pull_failed` with no deploy, whose `details.message` names the cause and says the running instances are unaffected. It reaches REST, GraphQL, MCP, webhooks and the dashboard Events feed, and sends one Image Pull Failed email worded for a running service. It is not repeated while the outage lasts, and a recovery clears the condition so a later outage reports again.
- The check never changes the running Deployment, and never runs for a service whose image is in bex's own registry.
- On production, a fixture service deployed from a `ttl.sh` image raises the event and the email after the image expires, and the service keeps serving.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect other than postgresql to w4`, 2026-11-26 (the aspect: deploying a prebuilt image, as [render.com/docs/deploying-an-image](https://render.com/docs/deploying-an-image) describes), item 4. Re-checked by hand at `origin/main` `28b4f18bbe` before filing: no open, blocked or done item on any workstream owns it.
- **Render:** "Render periodically checks image availability for active services. If an image is unavailable, Render can notify you even if your running services are unaffected. However, any action that requires a fresh image pull might fail until the image is reachable again." The Image Pull Failed event's details require only `message` and `imageURL`, not a deploy (`lego/backend/internal/api/openapi/render-public-api-1.json`).
- **bex today:**
  - Nothing checks a running service's external image; bex learns it is gone only when a pull fails.
  - When a pull of the live release does fail with no deploy open, w8/m156 t005 already records `image_pull_failed` with no deploy (`buildServedReleasePullFact`, `lego/backend/internal/store/reconciler.go:1871-1891`). This milestone reuses that fact shape for a failure found before any pull.
  - The email's lead says "could not pull its Docker image, so the new version did not start" (`lego/backend/internal/notifications/alert_email_worker.go:462-464`), which is wrong for a running service.
  - w8/blocked/m156 keeps bex's own registry from deleting a served build; this milestone covers external registries, which bex does not control.
- **Recommendation, not a user decision:** the interval (proposed: every 6 hours) and the rule "two consecutive not-found or refused answers" are bex's choice, set in t001; Render does not publish its own.
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first), and reliability: a service should not learn at 3 a.m., on a node replacement, that its image was deleted weeks ago.
- **Expected outcome:** a deleted image or an expired credential is reported while the service still runs, with time to fix it.
- **Why now:** bex replaces nodes and scales automatically, and each of those pulls the image afresh, so a silently deleted image turns into an outage at a random time. w4/m669 supplies the causes this check reports (t001 depends on w4/m669 t001).
- **Render parity:** included: events on REST, GraphQL, MCP and webhooks, the email and the dashboard change.

## Record — 2026-11-26

**Triage: work.** Every `bex today` pointer held at `origin/main` `c4431e3a1f`: nothing under `lego/` checked a running service's image (no availability check, no `ImageAvailable` condition), the only no-deploy `image_pull_failed` was `buildServedReleasePullFact`'s for a pull that had already failed, and the Image Pull Failed email's lead said "so the new version did not start". w4/m669 t001, which this depended on, shipped in `3bb4796980`.

**Render's reference, re-read 2026-11-26** ([deploying-an-image](https://render.com/docs/deploying-an-image)): "Render periodically checks image availability for active services. If an image is unavailable, Render can notify you even if your running services are unaffected." No interval or rule is published. Docker Hub: "Using GET emulates a real pull and counts towards the limit. Using HEAD won't" ([pull usage](https://docs.docker.com/docker-hub/usage/pulls/)).

**What shipped.**

- **t001, the design.** [ADR004 § A running image service's image is checked](../../../../docs/ADR004-app-deployment.md#a-running-image-services-image-is-checked-w4m671-2026-11-26): which services, which reference, which credential, the request, the 6-hour interval spread over ±10%, the two-refusals rule with a 15-minute confirmation, the load bounds, recovery and status ownership.
- **t002, the operator.** A new `app-image-check` controller (`lego/operator/internal/controller/image_check.go`) with its own App queue and one worker checks a running (or sleeping), unsuspended web service, private service or background worker whose served image (`status.image`) is a prebuilt image outside bex's registry. One `HEAD` on the manifest (`registry.CheckImageManifest`, which shares the metadata read's token exchange, redirect policy and SSRF-guarded client through a new `repoSession`), with the release's own credential, through a read gate of its own and spaced 2 s apart. A 404, 401 or 403 twice in a row sets `ImageAvailable` False with the cause in m669's words (`ImageNotFound`, `ImageCredentialRefused`, `ImageAccessRefused`); a 429, 5xx or network failure never does. One served answer turns it True. `status.imageCheck` keeps the schedule across restarts, and a runtime status retry keeps both from the fresh App. `BEX_IMAGE_CHECK_INTERVAL` (env, default `6h`, `0` off) is in `lego/operator/AGENTS.md`.
- **t003, bex-api.** `recordObservations` records one `image_pull_failed` per False transition (`availability:<app>:<transition>`), with no deploy, `imageURL` the reference checked and `details.message` the cause followed by the new `image_unavailable_serving` sentence ("… Running instances are unaffected, but any action that pulls the image again, such as a restart or a scale-up, will fail until the image is reachable again"). GraphQL's `messageTemplate` names the cause as its part (`gqlutil` reads a whole-reason `{cause}`), and the dashboard words it in place (the renderer now substitutes a worded hint or cause where the sentence puts it); all 8 languages. The email is its own `image_unavailable` stream (migration 0352) whose lead is "The Docker image of "<service>" can no longer be pulled from its registry." The projector's status watch counts `ImageAvailable` only while False.

**ttl.sh keeps the digest.** A tag pushed with `skopeo copy --all` to `ttl.sh/<name>:2m` answered 404 by tag after it expired, while its digest still answered 200 forty-five minutes later. A web or private service pins its digest and pulls it on a restart, so the check rightly reports nothing for it; a background worker pulls its tag. The walks therefore read the outage on a worker, with a web service beside it as the control.

**dev-4 (t004, first half).** The shared cluster's operator was rebuilt from this tree (`bash scripts/mock-cluster.sh operator`) with `BEX_IMAGE_CHECK_INTERVAL=2m` set for the walk and unset afterwards, and dev-4's bex-api was rebuilt from it (`dev-env.sh 4 up`). A disposable verified Kratos identity and its workspace (disposable) created a background worker `m671-worker` and a web service `m671-web` (both disposable) from `traefik/whoami:v1.10.4` copied to `ttl.sh/bex-m671-walk-<random>:15m`.

- Both reached Running and were checked at once: `ImageAvailable` True, `status.imageCheck` the tag (worker) and the pinned index digest (web), then every ~2 minutes.
- The tag expired at 20:08:47 UTC. The worker's next check (20:10:49) counted one refusal with the condition still True, and the confirming check (20:12:59) turned it False, `ImageNotFound`, "the image ttl.sh/bex-m671-walk-…:15m was not found in its registry: …". Its Deployment kept generation 1 and its ready instance, and the phase stayed Running.
- One `image_pull_failed` with no deploy, dated 20:12:59, answered the same `imageURL` and `details.message` (the cause, then "Running instances are unaffected, but any action that pulls the image again, such as a restart or a scale-up, will fail until the image is reachable again") on REST list, `GET /v1/events/{id}` (what a webhook receiver reads by `data.id`), MCP `list_events` and `get_service_event`, and GraphQL, whose `messageTemplate` was `image_unavailable_serving` with `image_not_found` as its part.
- Mailpit received one "Image pull failed: m671-worker": "The Docker image of "m671-worker" can no longer be pulled from its registry.", then the cause and the running-instances sentence, "At 2026-11-26 20:12:59 UTC." and the service's Events link. A third refusal (20:14:55) kept the transition: still one event and one email.
- The tag pushed again at 20:15:41: the next check (20:16:48) turned the condition True. After that copy expired too (20:30:41), a refusal at 20:31:04 and the confirmation at 20:33:05 recorded a second event, dated by its own transition. Its email waits in the stream's one-hour digest window, as every alert's second occurrence does.
- The web service stayed True throughout: ttl.sh kept serving its digest. Two of its checks met a 5-second timeout from the cluster ("context deadline exceeded") and were no verdict, retried without touching the count, as designed.
- The first outage and the recovery ran on the build before t006's simplification, and the second outage on the operator and bex-api rebuilt after it; the rule is the same in both. Removed afterwards: both services (`DELETE` 204 each), then the identity (the admin read answers 404).

**Render parity (t005).** REST, by id, MCP and GraphQL answer the event from one projection, and the webhook keeps Render's `{id, serviceId, serviceName}`, so a receiver reads the message by id, as on Render. The email and the Events row show the same sentence, the Events row worded in the dashboard's language. ADR018's Service events row ([services-lifecycle.md](../../../../docs/ADR018/services-lifecycle.md)) and notifications row ([platform-events-integrations.md](../../../../docs/ADR018/platform-events-integrations.md)) and [docs/render-artifacts/service-events.md](../../../../docs/render-artifacts/service-events.md) record what shipped. No drift beyond what Render does not publish: its interval, its rule and its exact event wording (`w10/blocked/002`), so bex's are recorded as bex's choice.

**Simplify (t006).** `/simplify` (reuse, quality and efficiency reviews):

- The registry read gate's acquire/release is one helper (`gatedRegistryRead`) for a deploy's metadata read and the check.
- The check reads through a gate of its own, so it never turns a deploy's read away, and a registry failing checks does not back deploys off. It is spaced 2 s apart by a blocking pacer, in place of a per-minute budget that requeued every due App each minute.
- An answer that is no verdict for good (a 400, a refused redirect) waits an interval, not 30 minutes. A failed status write requeues in a minute instead of erroring into an immediate repeat.
- The service-type switch is `App.RollsDeployment`, and the external-image decision moved into the target, so an App with nothing to check is never requeued.
- The two retry sites share `keepIndependentStatus`. The condition carries no generation.
- bex-api's status watch counts `ImageAvailable` only while False, so the True written on every new image wakes no projector pass. The fact names `status.imageCheck`'s image only.
- One alert email case covers both leads, and a redundant branch and the repeated Render quotes went.
- Not taken: moving `sniproxy.EnvDuration` into `types/env` for one caller.

**Test coverage (t007).** Reverting each guarded line failed its test:

- operator: the two-refusals count, the 404/401/403 set (a 429 added), the suspended skip, the stale-record drop, the runtime retry's carve-out, the confirmation wait, the credential-refused wording, the HEAD (a GET), the pacer;
- bex-api: the transition-keyed source key, the reconciler's call, the checked image, the email's classification and its lead, `gqlutil`'s whole-reason cause, the fingerprint's False-only rule, the alert event vocabulary, and migration 0352's CHECK (on a fresh database);
- dashboard: the in-place substitution.

A redundant `|| imageUnavailable` branch survived its revert and was removed. Suites: `make lint`, operator `make test`, the backend suite against real Postgres, OpenFGA and OpenBao, and the dashboard's `yarn lint` and the touched vitest files.

**Parked 2026-11-26: production half of t004.** The live check needs production to run `89fd36b809`. Once the pin carries it: in the bex-canary fixture workspace (`tea-daif693dqjvc73e7as3g`, the QA user's; `bash scripts/qa-login.sh`), create a background worker (disposable) from `ttl.sh/bex-m671-<random>:1h`, a `skopeo copy --all` of `traefik/whoami:v1.10.4`. Use a worker, not a web service: ttl.sh keeps an expired tag's digest, and a web service runs its pinned digest. Production checks every 6 hours (±10%) and confirms a refusal 15 minutes later, so after `ImageAvailable` reads True, park again with a `date gate` about 8 hours out. The worker that resumes reads `ImageAvailable` False (`ImageNotFound`), one `image_pull_failed` with no deploy whose `details.message` names the cause and says the running instances are unaffected, on REST, `GET /v1/events/{id}`, GraphQL (`messageTemplate` `image_unavailable_serving`) and MCP, and the "Image pull failed" email with its running-service lead if the QA mailbox is readable (say so if it is not). It confirms the worker still runs, deletes it, records the walk here, and closes out (t008).
