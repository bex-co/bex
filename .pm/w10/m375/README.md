# w10 · m375 — Auto-deploy reads what will actually happen, as on Render

**Worker:** worker10 **Goal:** a service or Blueprint whose repo cannot deliver pushes to bex is never stored or shown as auto-deploying, an explicit request to turn it on is refused with the reason and the way out, and the Deploys page's auto-deploy-off banner (w3/064) tells such a service that deploys are manual. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Read Render's contract: the auto-deploy rule, the Deploys-page banner, and what its API answers | 20m | — |
| [t002](t002.md) | A repo whose pushes can't reach bex gets auto-deploy off, and an explicit `yes` is refused on REST, GraphQL and MCP (absorbs w4/622) | 1h | t001 |
| [t003](t003.md) | A Blueprint on such a repo is created without auto-sync, and its preview says so (w4/622, round 258) | 45m | t002 |
| [t004](t004.md) | The dashboard offers no Auto-Deploy control for such a repo, and its rollback dialog promises nothing it can't do | 30m | t002 |
| [t005](t005.md) | For a repo whose pushes can't reach bex, the auto-deploy-off banner says deploys are manual and offers no turn-on link | 20m | t001, t004 |
| [t006](t006.md) | Render parity | 20m | t003, t005 |
| [t007](t007.md) | Simplify | 15m | t006 |
| [t008](t008.md) | Test coverage | 40m | t006, t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

Total: **9 tasks, about 4h20m**.

## Definition of done

On production, after the deploy that carries this milestone, as the QA user in `bex-canary`:

1. `POST /v1/services` for a public repo the workspace's GitHub App does not cover returns `autoDeploy: "no"`; `PATCH {"autoDeploy":"yes"}` returns 400 naming the reason on REST, and GraphQL `setAutoDeploy` and MCP `update_service` refuse alike.
2. A rollback on that service records no `auto_deploy_disabled` and its dialog promises no auto-deploy change.
3. A Blueprint preview for such a repo shows the manual-sync copy and the created Blueprint reads `autoSync: false`.
4. The service's Deploys page shows w3/064's auto-deploy-off banner in its manual-deploy form, without a turn-on link; a covered repo's service with auto-deploy off shows it with the link; an image service shows none; desktop and narrow-mobile.
5. A covered repo's new service still defaults to `autoDeploy: "yes"`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w10` (2026-11-12/13), filed by `/pmx them all to w10` on 2026-11-13, re-checked at filing against `origin/main` `3c6f11dde`, item 4. Absorbs [w4/622](../../w4/done/622.md) (rounds 248 and 258, 2026-11-13). The proposal also named [w3/064](../../w3/064.md), but w3's worker shipped its banner in `162cb9302` while this was being filed, so w3/064 stays in w3 and t005 only adds the uncovered-repo variant its coordination clause asked for.
- **Render:** [deploys](https://render.com/docs/deploys): "Auto-deploys require a connected Git provider. Services that use a prebuilt Docker image or a public Git repository URL must be deployed manually." [infrastructure-as-code](https://render.com/docs/infrastructure-as-code): Blueprint syncs follow pushes to a connected repository. Render changelog 2026-09-03: the new Deploys page has "A banner flagging when auto-deploys are disabled" (wording not published; w3/064 § Progress).
- **Open decision (recorded as the proposal's recommendation, not a user decision):** Render's API answer to an explicit `autoDeploy: "yes"` on a public-URL service was never captured (it needs a Render account, w10/002's gate). The recommendation is to refuse it with 400, following the prebuilt-image refusal (w4/418), because storing a value that can never take effect is what Render's rule forbids; t002 switches to Render's shape if t001 captures a different answer. The stored value is off either way.
- **Goal linkage:** Render parity first (ADR008); git push to deploy (`.pm/GOAL.md` item 3); ADR026 GitHub integration.
- **Expected outcome:** no service or Blueprint claims pushes will deploy when they cannot, on any surface.
- **Why now:** w4/622 was extended twice in one day (Blueprints too), and w3/064's banner (now live in code) would otherwise link those users to a setting they cannot use.
- **Render parity:** included (REST, GraphQL, MCP and dashboard change).

## Findings

None yet.
