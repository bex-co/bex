# w8 · m131 — The pre-deploy command is a paid-plan feature, as on Render

**Worker:** worker8 **Goal:** Setting a pre-deploy command on a free web service is refused on every surface with Render's rule ("The pre-deploy command is available for paid web services, private services, and background workers"), and a free downgrade is refused while one is set, the way maintenance mode already is. **Status:** todo

## Tasks (in order)

| id   | title                                                                   | est | depends_on                 |
| ---- | ----------------------------------------------------------------------- | --- | -------------------------- |
| t001 | Admission: refuse a pre-deploy command on Free (create, update, clone)   | 45m | —                          |
| t002 | Blueprint: `preDeployCommand` on a free service fails validate and apply | 30m | w8/m131/t001               |
| t003 | Plan change: refuse a free downgrade while a pre-deploy command is set  | 30m | w8/m131/t001               |
| t004 | Existing free services that already carry one                           | 30m | w8/m131/t001               |
| t005 | Dashboard: the pre-deploy field on a free service                       | 30m | w8/m131/t001               |
| t006 | Render parity: REST/GraphQL/MCP/dashboard and ADR018 agree              | 20m | w8/m131/t002, w8/m131/t003, w8/m131/t004, w8/m131/t005 |
| t007 | Simplify the changed code                                               | 15m | w8/m131/t006               |
| t008 | Test coverage: free vs paid, every writer, downgrade, Blueprint         | 30m | w8/m131/t006               |
| t009 | Close out the milestone                                                 | 30m | w8/m131/t007, w8/m131/t008 |

## Definition of done

Live on production after the release pipeline deploys bex-api and the dashboard, with owned fixtures deleted by ID with 404 proofs:

- `bex services update <free web srv-id> --pre-deploy-command "echo x" --confirm -o json` exits 1 with `received response code 400: … pre-deploy command is available only for paid …`, and the service's `preDeployCommand` is unchanged.
- `bex services create --type web_service --plan free --image docker.io/ealen/echo-server:0.9.2 --pre-deploy-command "echo x" …` exits 1 and creates nothing (list unchanged).
- The same commands with `--plan starter` are accepted by validation (checked with `dryRun`/Blueprint validate, no paid fixture bought).
- `bex blueprints validate` on a free service with `preDeployCommand` exits 1 with the same sentence at `services[0].preDeployCommand`.
- GraphQL and MCP refuse the same way; the dashboard shows the field disabled on Free with the paid-plan hint.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` loop sweep 8004, 2026-10-22/23 UTC: bex v0.3.3, pin Render CLI v2.27.0 `a764810a7682`, `https://api.bex.co/v1/`, `bex-canary` (`tea-daif693dqjvc73e7as3g`), human device login, non-TTY JSON. Evidence below.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). [render.com/docs/deploys](https://render.com/docs/deploys): "The pre-deploy command is available for paid web services, private services, and background workers." [ADR018](../../../docs/ADR018-render-parity.md)'s Pre-Deploy Command row (line 88) records no free-plan divergence. w8/172 noted "Render documents pre-deploy as paid-only" but filed nothing. bex already refuses Free maintenance mode and edge caching the same way.
- **Expected outcome:** the free tier offers what Render's does, and every writer gives one message.
- **Why now:** major. Customer-visible parity divergence on the free tier (a migration Job per deploy on free compute). Many recent board items exercise pre-deploy on Free fixtures, so the gap keeps spreading into tests and docs. Render parity task included: REST/GraphQL/MCP/dashboard surfaces change.

## Evidence (sweep 8004)

Owned fixture `srv-dbdb7ui8ohec73f9fu70` (`qa-20261022-8004-env2`, free web service, `docker.io/ealen/echo-server:0.9.2`), deleted at the end of the sweep.

```text
$ bex services update srv-dbdb7ui8ohec73f9fu70 --pre-deploy-command "echo predeploy-8004" --confirm -o json
exit 0
→ data.serviceDetails.plan = "free", data.serviceDetails.preDeployCommand = "echo predeploy-8004"
$ bex deploys create srv-dbdb7ui8ohec73f9fu70 --wait --confirm -o text
Deploy dep-dbdbea7vlr1c73fj1ah0 succeeded for service srv-dbdb7ui8ohec73f9fu70
$ bex logs -r srv-dbdb7ui8ohec73f9fu70 --limit 100 -o text --text predeploy-8004
2026-10-23 01:23:54  predeploy-8004
2026-10-23 01:21:34  predeploy-8004
```

Blueprint control: validating a manifest with one `type: web`, `runtime: image`, `plan: free` service carrying `preDeployCommand: echo migrate` returns `{"plan":{…},"valid":true}`, exit 0.

Controls that already follow Render on Free: `bex services update <free> --maintenance-mode` → 400 `maintenanceMode requires a paid web service plan`; `PATCH /v1/services/<free>` `{"serviceDetails":{"cache":{"profile":"origin-controlled"}}}` → 400 `edge caching is not available for free web services; choose a paid plan first`.

## Root cause

- `lego/backend/internal/apps/settings_check.go:58` `preDeployCommandApplies` checks only the service type (not cron, not static). `checkPreDeployCommand` (`settings_check.go:190`) adds the protected-environment gate, with no plan check. Create uses the same type-only test at `service.go:3114`. `SetPreDeployCommand` (`service.go:4235`) and the settings PATCH path (`settings.go:382`) go through `checkPreDeployCommand`.
- The maintenance precedent: `validateMaintenanceEligibility` (`service.go:819`) and `SetPlan`'s free-downgrade refusal (`settings.go:602`).

## Fix (target)

- One predicate, e.g. `preDeployCommandAllowed(svcType, tier)`, used by create, update/PATCH, clone (`--from`), GraphQL, MCP and the Blueprint compiler. Message: `pre-deploy command is available only for paid web services, private services, and background workers; choose a paid plan first`.
- Clearing it (empty string) is allowed on any plan, so a free service can always remove one.
- `SetPlan` to free refuses while a pre-deploy command is set, like maintenance mode. A combined PATCH that clears it and downgrades applies the clear first (`settings.go:57-114` ordering).
- t004 (existing free services that already have one): inventory them. The parity-sufficient choice is to keep running their command (no silent skip of a migration) until the owner edits or clears it. The next write on such a service must satisfy the new rule. Record the count and this transition in ADR018.

## Blast radius

`preDeployCommandApplies` callers (3: `service.go:3114`, `settings_check.go:196`, plus `checkPreDeployCommand` callers `service.go:4244`, `settings.go:382`), GraphQL `setPreDeployCommand`/create, MCP create/update tools, the Blueprint compiler, clone (`services create --from` copies the source's command), dashboard create and settings forms. Every workspace, no allowlist.

## Unverified

- Render's exact API error text and status for this case (no Render account). The target uses 400 like bex's maintenance and edge-cache refusals.
- What Render does to a paid service with a pre-deploy command that is downgraded to Free.

## Dedupe

Searched `.pm` open and done, ADR018 and DO_NOT_DO for pre-deploy + free/paid: only the unfiled remark in w8/172. No open item.
