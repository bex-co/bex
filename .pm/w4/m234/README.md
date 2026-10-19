# w4 · m234 — A save on a suspended service is a save, never a deploy that cannot run

**Worker:** worker4 **Goal:** saving configuration on a suspended service never claims a deployment and never files a deploy that can only be canceled. The save stands, every surface says it waits for resume and a deploy, and the dashboard offers only the modes that can work. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Backend: a deploying environment save on a suspended service stages like Save only and says why | 45m | — |
| [t002](t002.md) | Environment groups: report a suspended linked service as pending, not rolled | 40m | w4/m234/t001 |
| [t003](t003.md) | Dashboard: gate the deploying save modes and the pending notice on suspension | 45m | w4/m234/t001 |
| [t004](t004.md) | Shared rollout tracker: no deploy row for a rolling write on a suspended service (blast radius) | 60m | w4/m234/t001 |
| [t005](t005.md) | Render parity | 20m | w4/m234/t002, w4/m234/t003, w4/m234/t004 |
| [t006](t006.md) | Simplify | 15m | w4/m234/t005 |
| [t007](t007.md) | Test coverage | 45m | w4/m234/t005 |
| [t008](t008.md) | Closeout | 25m | w4/m234/t006, w4/m234/t007 |

Eight tasks, 295m (4h55m). One **major** finding. This milestone schedules the work only.

## Definition of done

Replay [finding.md](finding.md) with the [recorded probes](evidence.json) on a fresh owned Free image web service (`docker.io/traefik/whoami:v1.10`, `WHOAMI_PORT_NUMBER=3000`) after the deploy. Suspend it through Settings → Suspend first.

- **Deploy mode saves and says why.** Environment → Edit → add a variable → the save control.
  - The primary button reads Save only. Save and deploy and Save, rebuild, and deploy are disabled with "Unavailable while the service is suspended. Resume it first." (the Manual Deploy reason).
  - Over GraphQL, `patchServiceEnvironment(saveMode:"deploy")` answers `rolledOut:false` and a product-English `rolloutError` naming the suspension and the resume-then-deploy step: no `conflict:` prefix and no raw `srv-` id. The variable is listed afterwards.
  - REST `PATCH /v1/services/{id}/environment` and MCP `patch_service_environment` answer the same.
  - `GET /v1/services/{id}/deploys` gains **no** row, and Events gains no `deploy_started`.
- **Rebuild mode (control) keeps its shape** (`rolledOut:false`, no row) and its `rolloutError` uses the same sentence.
- **Notice.** While suspended, the header and `/env` notice says to resume and then deploy, not "Use a standard deploy".
- **Sibling writers open no row.** REST `PUT /v1/services/{id}/env-vars/{key}` and `PATCH /v1/services/{id}` `{"serviceDetails":{"healthCheckPath":"/"}}` on the suspended service each succeed and add no deploy row.
- **Resume, then deploy.**
  - Resume serves the prior live release: Revision unchanged and `undeployedChanges:true`.
  - Either no deploy row opens, or exactly one row with trigger `service_resumed` that carries the saved changes (t004 records which).
  - Manual Deploy → Deploy latest image goes Live and `undeployedChanges` becomes `false`.
- **Control — not suspended.** The same Save and deploy on the running service still opens one config-change row that goes Live, toasting "Environment saved and deployment started".
- **Env groups.** A deploy-mode group save with one suspended and one running linked service lists the suspended one in `pendingServiceIds`. Only the running one gains a row.
- **Checks pass.** Backend `go test ./...` (real-DB gate), dashboard `yarn lint` and `yarn test`.
- **Cleanup.** No fixture remains, and only that run's session is revoked.

## Source + Goal linkage

- **Source:** live functional `/qa-find-bugs w4`, pass 9d0542, 2026-10-19 UTC, `muse.env`, workspace `bex-canary`. See [finding.md](finding.md) and [evidence.json](evidence.json); screenshots `.playwright-mcp/qa-env-1.png` and `qa-env-2.png` are local to that session. Production was pinned to `69e925197b39`, and research is on main with no `secrets`, `rollout` or environment-editor change since the pin.
- **Goal linkage:**
  - [ADR004](../../../docs/ADR004-app-deployment.md): the deploy lifecycle and its truthful history.
  - [ADR006](../../../docs/ADR006-bex-api.md): one environment-save core across REST, GraphQL and MCP; legacy env-var writes roll immediately.
  - [ADR008](../../../docs/ADR008-vision.md): truthful product state.
  - It completes the suspended gate that `deployActions`, Trigger and Rollback already apply.
- **Expected outcome:**
  - An owner who edits configuration while a service is suspended is told the change waits for resume and a deploy, not that a deployment started.
  - The deploy history and the event and webhook stream gain no canceled deploys from those saves.
- **Why now:**
  - The false success hides a real trap: after Resume the service runs the old values (a rotated credential, a changed flag) until someone deploys again.
  - Suspending to make a fix and then saving it is the normal recovery workflow.
  - The rebuild mode next to it already does the right thing, so the fix is small and well bounded.
- **Render parity included:** REST, GraphQL, MCP and dashboard behavior all change. t005 compares Render's documented behavior: API env updates never deploy on their own, and "Save and deploy" redeploys the existing build. Render documents nothing for a suspended service, so t005 records bex's choice as a documented divergence or parity.
- **Relationship:** w4/m171 and w5/m114 (both blocked) own a suspend that interrupts a running deploy and what Resume serves afterwards. This milestone must keep their guarantees: "Canceled: the service was suspended" for an interrupted deploy, and Resume never rolling a release without a deploy row.
- **Unverified:**
  - MCP `patch_service_environment`.
  - The env-group deploy mode with a suspended linked service (code-read).
  - Webhooks for the canceled phantom rows.
  - The dashboard rebuild-mode toast text.
  - Settings fields other than `healthCheckPath`.
