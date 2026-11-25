# w4 · m630 — bex's skills pack follows render-oss/skills again, and a weekly job says when it falls behind

**Worker:** worker4 **Goal:** `bex skills install` installs skills derived from render-oss/skills' current main (Render's Blueprint plan → approve deploy flow over MCP and CLI), no skill sends agents to a tool bex removed, and a weekly job fails the moment upstream moves again. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Re-derive the pack at render-oss/skills `841f4e3` | 1h | — |
| [t002](t002.md) | Port #81 into the files bex overrides, and stop sending agents to `get_selected_workspace` | 1h | t001 |
| [t003](t003.md) | A weekly job fails when render-oss/skills moves past the pin | 1h | t001 |
| [t004](t004.md) | Walk a skill-driven Blueprint deploy on production with a `bex` built from main | 45m | t002, t003 |
| [t005](t005.md) | Render parity | 25m | t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 30m | t005, t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

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
