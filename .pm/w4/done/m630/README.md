# w4 · m630 — bex's skills pack follows render-oss/skills again, and a weekly job says when it falls behind

**Worker:** worker4 **Goal:** `bex skills install` installs skills derived from render-oss/skills' current main (Render's Blueprint plan → approve deploy flow over MCP and CLI), no skill sends agents to a tool bex removed, and a weekly job fails the moment upstream moves again. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Re-derive the pack at render-oss/skills `841f4e3` — **DONE** | 1h | — |
| [t002](done/t002.md) | Port #81 into the files bex overrides, and stop sending agents to `get_selected_workspace` — **DONE** | 1h | t001 |
| [t003](done/t003.md) | A weekly job fails when render-oss/skills moves past the pin — **DONE** | 1h | t001 |
| [t004](done/t004.md) | Walk a skill-driven Blueprint deploy on production with a `bex` built from main — **DONE** | 45m | t002, t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 25m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | t005, t006 |
| [t008](done/t008.md) | Closeout — **DONE** | 10m | t007 |

Total: **8 tasks, about 5h05m**.

## Definition of done

1. `packgen.UpstreamCommit` is render-oss/skills `841f4e3` (or a later reviewed head) and the pack equals what `sync-skills.sh` generates from it.
2. Every `lego/cli` test passes, including the 20-skill count and the content lint.
3. No pack file names `get_selected_workspace` or a `v2.x` CLI floor.
4. bex-deploy, bex-cli and bex-mcp guide the MCP and CLI create → plan → approve flow, and every tool and argument they name is served by bex.
5. The weekly job fails on an unreviewed upstream skills commit and passes at the new pin.
6. The production walk is recorded; the CLI release is done if the user authorized it, otherwise recorded as pending the user.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one more aspect of render parity for w4`, 2026-11-24 (the aspect: how a coding agent connects to bex, as Render's agent docs describe), each item re-verified by an independent agent on 2026-11-24/25 before filing; filed by `/pmx them all to w4` at `origin/main` `50a5ddfdfc`.
- **Evidence (verified 2026-11-24 at `2f964da9e8`):** the pin `33801b5` is five upstream commits behind; #81 (`0ec9e6c`, 2026-11-20) is the one that changes skills bex keeps. `bex-deploy/SKILL.md` (348 lines) never names the Blueprint tools bex serves. Generating at `841f4e3` into `/tmp` succeeds and shrinks bex-deploy to the create → plan → approve flow, but trips the content lint (`v2.35.0`, a mis-rewritten `render-mcp-server` link), and the overrides keep the old workspace guidance, including `get_selected_workspace`, which bex removed in w1/m55. No job watches render-oss/skills.
- **Goal linkage:** ADR008 AI-native; Render parity of the agent skills Render ships (`render.com/docs/llm-support`).
- **Expected outcome:** agents using bex's skills deploy Blueprints the way Render's now do, and upstream drift is caught within a week.
- **Why now:** the drift is silent and growing, the pack already sends agents to a removed tool, and w4/m631 copies this pack into the Claude Code plugin.
- **Render parity:** included (customer-facing skill content naming MCP and CLI behavior).
- **Open user decision:** the CLI release that carries the pack to users (`/release cli`) is the user's; t004 records it as pending unless the user approves it.

## Live check — 2026-11-25 15:26–15:32 UTC: passed

`bex` built from `origin/main` `18f6d3388b` (the commit that shipped t001–t003; `bex vdev-18f6d3388b`, Render CLI v2.38.0 pin). In a sandboxed `HOME`, `bex skills install --tool claude --scope user --confirm` installed 20 `bex-*` skills; the installed `bex-deploy` is byte-identical to the pack, and no installed file names a session workspace tool or a `v2.x` floor. Each walk followed `bex-deploy` step by step: validate, create with an explicit branch and `autoSync: false` (main moves constantly), review `planningOutcome`, approve, then follow each service's deploy to a terminal state and check the app.

- **Workspace.** `createWorkspace` answered `PAYMENT_REQUIRED` for the QA user, Hobby included (as in w21/m416 t013), so no disposable workspace could be made. Both walks ran in `bex-canary` (`tea-daif693dqjvc73e7as3g`, the QA user's fixture workspace), as w2/m256 t007 did, with a run-minted API key. The workspace's four pre-existing services were snapshotted and never touched.
- **MCP** (`examples/blueprint-refs/render.yaml`, every call with `workspaceId`): `create_blueprint` → `exs-dc3ft415k4kc73dtajj0`, `status: created`, `planningOutcome: planned`, no notices, `sync.requiredManualEnvVars` = `MANUAL_SECRET` on `blueprint-refs-app-262w` (`create_new` suffix), `sync.dashboardUrl` on `dashboard.bex.co`; the plan created one environment group and three services. `approve_blueprint` with the reviewed `syncId` and `manualEnvVars` → sync `success`. `get_blueprint` → `paused` (auto-sync off) and the four managed resources. `list_deploys`: the static site `live` at 15:28:44, both Docker services `live` by 15:29:37. The app's report shows the group, peer and site references resolved, the generated secret set and `MANUAL_SECRET` set (21 bytes, the value supplied at approval). `plan_blueprint` then answered `no_changes`.
- **CLI** (`examples/retained-env/render.yaml`, `BEX_WORKSPACE` set, `bex workspace current` confirmed `bex-canary`): `bex ea blueprints create --repo … --branch main --path … --auto-sync=false --existing-resources create_new -o json` → `exs-dc3ftu15k4kc73dtajug`, `planned`, no manual values, `meta.next` naming `bex ea blueprints approve …`. `bex ea blueprints approve … -o json --confirm` → sync `success`. `bex deploys list` → `retained-env-fvln` `live` at 15:30:36, and its URL answers the whoami page. `bex ea blueprints plan` then answered `no_changes`.
- **Cleanup.** Both Blueprints, the four services and the environment group were deleted (204 each) and each detail read answers 404; `bex-canary` lists exactly its four pre-existing services. The run key was revoked (204) and the QA session logged out.
- **Release.** Not run: the CLI release that carries this pack to users is **pending the user's `/release cli`** (not authorized for this milestone). Until then `bex skills install` and `bex skills update` on a released `bex` install the `33801b5` pack.
