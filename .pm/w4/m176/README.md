# w4 · m176 — Protected environments: a deploy with `imageUrl`/`commitId` or a rollback swaps the running code without the "repoint" confirmation

**Worker:** worker4 **Goal:** on a member of a `protectedStatus: protected` environment, every verb that selects which code runs needs the `repoint` confirmation that ADR032 (w4/m126) already requires of `image`/`repo`/`branch` edits. That includes a deploy trigger carrying `imageUrl` or `commitId`, and a rollback. **Status:** todo

## Tasks (in order)

| id   | title                                                                     | est | depends_on   |
| ---- | ------------------------------------------------------------------------- | --- | ------------ |
| t001 | Guard `Trigger` overrides and `Rollback` with the protected confirmation  | 45m | —            |
| t002 | Project `protected_confirmation_required` on deploy/rollback actions; dashboard confirm flow | 45m | w4/m176/t001 |
| t003 | Record the decision in ADR032's service table                            | 15m | w4/m176/t001 |
| t004 | Render parity                                                             | 20m | w4/m176/t002, w4/m176/t003 |
| t005 | Simplify                                                                  | 15m | w4/m176/t004 |
| t006 | Test coverage                                                             | 45m | w4/m176/t004 |
| t007 | Closeout                                                                  | 10m | w4/m176/t006 |

## Definition of done

Live on an owned Free `traefik/whoami` web service in a fresh project whose environment is `protectedStatus: protected` (two prior deploys so a rollback target exists; everything deleted afterwards):

- **Override deploy is guarded.** `POST /v1/services/<id>/deploys` `{"imageUrl":"docker.io/traefik/whoami:v1.10.1"}` without a confirmation → 400 with the named protected-environment error and the exact `sudo repoint service <name>` phrase (the shape `PATCH … image` already returns). No deploy row is created and the running image is unchanged. With `?confirm=<phrase>` → 201 and it goes Live on `v1.10.1`. Same for `commitId` on a repo-backed service.
- **Rollback is guarded.** `POST /v1/services/<id>/rollback` `{"deployId":"<older deploy>"}` without the phrase → refused the same way; with it → 201 and Live on the older artifact.
- **Bare redeploy stays exempt.** `POST /v1/services/<id>/deploys` `{}` (no override) → 201 without a phrase. It redeploys the configured artifact, whose staging is already guarded at the setter.
- **Controls still hold.** `suspend`, `DELETE` and a start-command PATCH stay refused without the phrase; `restart` and env-var writes stay unguarded (ADR032's recorded exemptions).
- **Unprotected services are untouched.** The same override deploy and rollback on a service in no environment succeed with no phrase and no new argument.
- **All surfaces agree.** GraphQL deploy-trigger/rollback mutations and the MCP deploy/rollback tools refuse and accept on the same terms. `deployActions` reports the `rollback` decision with `precondition: "protected_confirmation_required"` on a protected member. The dashboard's Rollback dialog asks for the phrase rather than failing at submit.

## Source + Goal linkage

- **Source:** infinite `/qa-find-bugs` loop60, 2026-10-05 UTC, muse.env, `bex-canary`, pin `c3dc732e2`. Fixtures: project `qa-20261005-l60-proj` (`prj-db237lqgg1mc73cobhc0`), environment `production` (`evm-db237lqgg1mc73cobhcg`) set `protectedStatus: protected` (GraphQL read-back `protected`), service `qa-20261005-l60-prot` (`srv-db237nqgg1mc73cobhdg`). All deleted afterwards.
  - **Guard armed (controls):** `POST …/suspend` → 400 "… is a member of a protected environment; retry with confirm=…". The same for `DELETE /v1/services/<id>` and `PATCH … {"serviceDetails":{"envSpecificDetails":{"dockerCommand":…}}}`. `POST …/restart` → 200 and `PUT …/env-vars/QAX` → 200, both ADR032 exemptions.
  - **Override deploy unguarded:** `POST /v1/services/srv-db237nqgg1mc73cobhdg/deploys {"imageUrl":"docker.io/traefik/whoami:v1.10.1"}` → **201** `dep-db238rnl9hss73chc1gg`, `image.ref: docker.io/traefik/whoami:v1.10.1`, `trigger: api`. It was `live` at 23:37:17Z, deactivating the `traefik/whoami:latest` deploy.
  - **Rollback unguarded:** `POST …/rollback {"deployId":"dep-db238dqgg1mc73cobhi0"}` → **201** `dep-db239j2gg1mc73cobhk0`, `live` at 23:38:00Z, swapping `v1.10.1` back to `latest`. An earlier rollback (`dep-db238dnl9hss73chc1fg`) was also accepted without a phrase, then superseded.
- **Root cause:**
  - `lego/backend/internal/deploys/service.go:509-522` `Trigger` already recognises that `imageUrl`/`commitId` "SELECT the executable content this deploy runs" and raises authorization to create-like (codex round-5 F2), but never calls the protected-environment guard.
  - `Rollback` (`:1025-1045`) authorizes `RelCanCreate`, checks deleting/billing/suspended, and likewise never consults protection.
  - The guard lives only in the apps package (`lego/backend/internal/apps/protection.go:128`, `requireUnprotected` and `ProtectedConfirmation("repoint", name)`). The deploys package has no call to it, nor to `core.RequireEnvironmentConfirmation` (`lego/backend/internal/core/confirmation.go:86`), which the Postgres and Key Value guards use (`postgres/protection.go:35`, `keyvalue/protection.go:56`).
  - The action projection (`lego/backend/internal/deploys/actioncaps.go:106-133`) computes only suspended/billing/eligibility preconditions for `deploy`/`rollback`, so UIs cannot know a phrase is needed.
- **Why this is a gap, not a decision:** ADR032 §"What `protectedStatus` covers on a service" (w4/m126) says "for a service, 'what it is' is the code it runs" and lists its deliberate omissions (Resume, restartServer, webhook auto-deploy, maintenance-off, schedule-only, scale, plan/idle, reversible config, env vars). Neither override deploys nor rollback is on either list. m126's DoD probed only setters (`setImage` etc.), never the deploy verbs.
- **Fix target:** in `Trigger`, when `ImageURL != "" || CommitID != ""`, and in `Rollback`, require `ProtectedConfirmation("repoint", name)` through the shared core confirmation (fail closed on lookup errors). Thread `confirm` on REST (`?confirm=`), GraphQL and MCP like every other guarded verb. A bare trigger stays lifecycle and unguarded.
- **Blast radius:** `Trigger` callers are REST `POST /v1/services/{id}/deploys`, GraphQL deploy-trigger, MCP deploy-trigger and the dashboard Manual Deploy menu ("Deploy a specific commit"/image). `Rollback` callers are REST/GraphQL/MCP rollback and the dashboard deploy-row Rollback. The webhook auto-deploy and deploy-hook paths do not reach `Trigger` with overrides and stay exempt; t001 must confirm that by grep.
- **Adjacent classes:** unauthenticated/forbidden stay ahead of the protection check (authorization first, so the phrase is not an oracle to non-members). A wrong phrase gets the same refusal as a missing one. Protection-lookup failure refuses (fail closed, matching delete/suspend).
- **Governing docs:** ADR032 (environments, protected verbs), ADR004 (deploys), ADR018 deploy trigger/rollback rows.
- **Goal linkage:** ADR008 safe production operations; the protected-environment promise.
- **Expected outcome:** the code a protected service runs cannot be swapped by any verb without the typed confirmation.
- **Why now:** the image setter is guarded, so the protection reads as complete, but the deploy endpoint does the same swap in one call, and it applies immediately rather than on the next deploy.
- **Render parity:** included (t004). REST/GraphQL/MCP/UI semantics change on protected members. Render limits protected-environment actions to admins rather than a typed phrase; bex's phrase model is ADR032's documented divergence.
- **Unverified:** `commitId` on a repo-backed service (reasoned from the same branch); the GraphQL/MCP trigger and rollback entry points and the dashboard dialogs (REST probed only); the Blueprint stack-apply override (`applyCreate`), already guarded per ADR032, not re-probed.
- **Severity:** major (a safety guard is bypassable; no data loss observed).
