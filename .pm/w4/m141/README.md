# w4 · m141 — Rolling back a static site does nothing: an enabled Rollback, a confirm dialog, then silence

**Worker:** worker4 **Goal:** a user who clicks Rollback either gets a rollback or is told, before and at dispatch, exactly why they cannot. For static sites, rollback works the way Render's does (re-publish an earlier revision), or the control is honestly unavailable with that reason **Status:** todo

## Tasks (in order)

| id   | title                                                                                               | est | depends_on       |
| ---- | --------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Decide and implement static-site rollback: re-publish the target's revision, or refuse it by name | 60m | —                |
| t002 | Dispatch recheck applies the same selected-row eligibility as the button and surfaces every refusal | 40m | —                |
| t003 | Name each row's Rollback control after its deploy                                                  | 15m | —                |
| t004 | Render parity across REST / GraphQL / MCP / UI                                                     | 20m | t001, t002, t003 |
| t005 | Simplify                                                                                            | 15m | t004             |
| t006 | Test coverage                                                                                       | 30m | t004             |
| t007 | Closeout                                                                                            | 10m | t006             |

## Definition of done

Each bullet was probed at filing (pass 178, 2026-09-26) on a throwaway no-build static site `qa-20260926-dep` (`srv-darnbvod0qnc73d79s60`, `https://github.com/bex-co/bex`, root `examples/static-site`, publish `.`), deleted the same pass. It had three deploys: `…79s6g` (create, `80b423bc`, deactivated), `…79s7g` (Restart, `80b423bc`, deactivated), and `…79s9g` ("Deploy a specific commit" `28aa2ab7`, live).

- **Rollback either happens or is refused out loud.** On `/static/<id>/deploys`, click **Rollback** on the `…79s7g` row, then **Proceed**. At filing: the dialog closed, **no mutation was sent** (the only request after Proceed was the `DeployActions` recheck), no toast appeared, and a minute later `deploys(serviceId:)` was unchanged, with `…79s9g` still live. Done means a rollback deploy (`trigger: rollback`) goes live and serves the earlier revision, or the user sees the refusal with its reason: before clicking (disabled with the reason, as `PermissionTooltip` does elsewhere), and never as a silent no-op after Proceed.
- **The capability and the control agree.** At filing, `deployActions(serviceId:)` answered `{action: "rollback", outcome: "allowed", precondition: "no_eligible_rollback_target"}`, while two rows each rendered an enabled `button "Rollback"`.
- **Rollback controls say which deploy they target.** At filing both were named just "Rollback".

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` pass 178, 2026-09-26 (w4-targeted, `muse.env` credentials), journey 3 (deploys) on a static site. Captured request/response after Proceed:

  ```
  POST /graphql  [{"operationName":"DeployActions","variables":{"serviceId":"srv-darnbvod0qnc73d79s60"}, …}]
  → [{"data":{"deployActions":[…,{"action":"rollback","outcome":"allowed","precondition":"no_eligible_rollback_target","reason":null}]}}]
  ```

  No `RollbackService` mutation followed.
- **Root cause (two defects that compound):**
  1. **No static deploy is ever eligible.** `lego/backend/internal/deploys/actioncaps.go:32-34` `RollbackEligible` requires `d.ResolvedImage != ""`, and a no-build static publish (ADR029: "clones the repo and publishes rootDir/staticPublishPath as-is") has no image, so every static deploy fails it. Static-with-build (an image exists) is unverified.
  2. **The button and the dispatch use different rules.** `dashboard/src/features/deploys/components/deploy-actions.tsx:100-107` deliberately overrides the service-wide `no_eligible_rollback_target` for a row whose status is rollbackable (`decisionForSelectedRollback`, per `w6/m143/t003`; the status rule is just `status === "deactivated"`, `features/deploys/lib/deploy-status.ts:47-49`), so the button is enabled. At dispatch, `handleConfirm` calls `recheckBeforeDispatch` (`features/capabilities/hooks/use-bound-action-confirm.ts:140-148`), which judges the **raw** decision without that override, finds a precondition, and returns `{ok:false}`. `handleConfirm` then `return`s with no toast. So even a genuinely eligible older row on a service whose latest-20 summary has no target is silently refused, which undoes `w6/m143/t003`'s intent at the last step.
- **Target behavior:** exact-target eligibility is the one rule used for enabling **and** for the dispatch recheck. It is evaluated per row (the backend verb already validates the named target), and every refusal reaches the user with its reason. For static sites, Render's dashboard offers rollback to an earlier deploy, so the preferred fix is to record the published revision as the rollback target (t001 decides, with evidence from `internal/publish`). If that is out of scope, the control is disabled with an explicit "static sites without a build can't be rolled back" reason.
- **Adjacent copy (same pass):** the rollback dialog says "redeploy from the image used in this deploy", and the Restart dialog says "New instances replace the old ones", on a site that has neither image nor instances. t001 aligns the copy with whichever behavior it picks.
- **Goal linkage:** ADR004 (deploys), ADR029 (static sites), ADR018 deploy parity, `w6/m143` (capability-gated actions), `w4/051` (rollback targets).
- **Expected outcome:** Rollback never silently does nothing.
- **Why now:** the silent path defeats a capability design that was just built (`w6/m143`) and affects every static site. A user who believes they rolled back a broken site is still serving it.
- **Render parity task included:** UI behavior and possibly the rollback verb's static-site support change.

## Control (pass 179, 2026-09-26)

The same Rollback flow works on an **image-backed** web service. On throwaway `qa-20260926-rb` (`srv-darnj0psmc7s73cq5gu0`, Existing Image `docker.io/traefik/whoami:v1.10.1`, deleted the same pass): Update Source to `:v1.10` → Deploy latest image → Rollback on the first deploy → Proceed. The dispatch sent `DeployActions` then `RollbackService`, the toast read "Rollback triggered.", and REST `GET /v1/services/{id}/deploys` showed `…q5h1g:live:rollback:docker.io/traefik/whoami:v1.10.1` above `…q5h00:deactivated:api:…:v1.10`. The failure in this milestone is therefore specific to deploys without a `ResolvedImage`, as the root cause states. The same pass noticed that an image-backed service's Manual Deploy menu also offers "Clear build cache & deploy" with no build to clear. t001 should decide it alongside the static-site copy.

## Unverified

- A static site **with** a build command (which produces an image) was not exercised. Its deploys may be eligible, and whether rollback then re-publishes correctly is t001's to prove.
- Whether the backend `Rollback` verb, called directly on a no-build static deploy, answers a named 409 (expected, from the same `ResolvedImage == ""` arm noted in `w4/051`) was not probed.
