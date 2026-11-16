# w4 · m431 — Deploying a specific commit from the dashboard or a deploy hook turns auto-deploy off in the same write and records `manual_deploy`, as on Render

**Worker:** worker4 **Goal:** a commit pinned through the dashboard or a deploy hook's `ref` stays pinned: the same write turns auto-deploy off, the event reads Render's `manual_deploy`, and REST and MCP triggers keep auto-deploy as Render's API does. **Status:** blocked (t001–t003, t005–t007 done; t004 waits for the production deploy carrying this change)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | One trigger verb that pins a commit and turns auto-deploy off in the same write, recording `manual_deploy` — **DONE** | 45m | — |
| t002 | A deploy hook called with `ref` pins the commit and turns auto-deploy off — **DONE** | 25m | t001 |
| t003 | GraphQL `triggerDeploy(disableAutoDeploy:)` and the dashboard's Deploy a specific commit in one mutation — **DONE** | 40m | t001 |
| t004 | REST and MCP keep auto-deploy; live check on production | 30m | t002, t003 |
| t005 | Render parity — **DONE** | 20m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 30m | t005 |
| t008 | Closeout | 10m | t006, t007 |

## Definition of done

1. On production in `bex-canary`, with an owned free web service deleted afterwards (by-id `GET` 404), t004 records: a deploy-hook call with `ref` and the dashboard's Deploy a specific commit each leave `autoDeploy: false` and one `auto_deploy_disabled` event with `reason: manual_deploy`, and a later push opens no deploy.
2. A REST `POST /v1/services/{id}/deploys {commitId}` and MCP `trigger_deploy` with a commit leave auto-deploy on and record no such event.
3. The dashboard sends one mutation for a specific-commit deploy; no follow-up `setAutoDeploy` write exists.
4. `core/audit.go` no longer says `manual_deploy` is never written; ADR018's Trigger a deploy and Service event types rows record the rule.
5. `cd lego/backend && go test ./...`, `make test` and `make lint` in `lego/operator/`, and dashboard `yarn lint` + `yarn test` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for one aspect of render parity` (the Deploys aspect), 2026-11-16, item 1; filed by `/pmx them all to w4` the same day.
- **Render reference:** render.com/docs/deploys § Manual deploys, read 2026-11-16: "Deploying a specific commit through the Dashboard or a deploy hook disables auto-deploys. The CLI and API don't."
- **bex today:** the deploy hook's `ref` (`lego/backend/internal/deploys/deployhook.go:378`) leaves auto-deploy on. The dashboard pin (w1/m148 t002) triggers, then turns auto-deploy off in a second write (`manual-deploy-button.tsx:188-191`), so the event reads `setting_change` and a push between the writes can still deploy the branch head. `core/audit.go:636-638`: "bex has no path that turns auto-deploy off on a manual deploy, so Render's manual_deploy is never written" (w3/m317).
- **Goal linkage:** ADR008 Render parity first; ADR018 § Deploys (Trigger a deploy, Secret deploy hook) and the Service event types row.
- **Expected outcome:** a CI job that pins a commit through a deploy hook is not overwritten by the next push, and the event reads as it does on Render.
- **Why now:** w3/m317 left this as the one Render reason bex never writes, and rollback already has the same one-write mechanism to reuse.
- **Render parity:** included: the change touches the deploy-hook REST route, GraphQL, the events read on REST/GraphQL/MCP, and the dashboard.
