# w3 · m303 — Render CLI v2.36.0: adopt it, match its sandbox contract, and notice the next release

**Worker:** worker3 **Goal:** bex answers the current official Render CLI the way Render does, and the next upstream CLI release is reported within a week **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Bump the upstream Render CLI pin to v2.36.0 | 45m | — |
| t002 | One sandbox timeout contract for every workspace | 1h | — |
| t003 | Name the v2.36 sandbox-group routes bex does not offer | 45m | t001 |
| t004 | Ended sandboxes report what the v2.36 CLI explains | 45m | t001 |
| t005 | Walk the CLI checklist with the unmodified v2.36.0 binary on production | 1h | t002, t003, t004 |
| t006 | The drift job notices a newer Render CLI and in-operation schema changes | 1.5h | — |
| t007 | Render parity | 30m | t005, t006 |
| t008 | Simplify | 20m | t007 |
| t009 | Test coverage | 40m | t007 |
| t010 | Closeout | 10m | t008, t009 |

## Definition of done

- `lego/cli` pins Render CLI v2.36.0 on main and its suite is green (the `bex-cli` release itself is the user's `/release cli`)
- On production with the unmodified v2.36.0 binary: a Hobby `--timeout=0` sandbox reads back `timeoutSeconds: 0`; `--timeout=30` answers `SANDBOX_TIMEOUT_INVALID`; `sandbox-groups delete`, `default set` and `credentials` print the named refusal; an ended sandbox shows the ended-and-when explanation; results recorded in `docs/cli-compatibility-checklist.md`
- ADR014 § Lifetime bound and ADR018's sandbox rows describe the uniform contract
- The drift job's tests show it reporting a newer render-oss/cli release and a changed in-operation schema

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w3` 2026-11-09/10, item 2 (filed by `/pmx them all to w3`); render-oss/cli v2.36.0 (2026-11-06) and commit `c280b0b66`; render.com/docs/sandboxes-limits; `lego/backend/internal/sandbox/lifetime.go:33-75`, `notoffered.go:37-50`; `lego/cli/UPSTREAM_RENDER_CLI.md`
- **Goal linkage:** Render parity first (ADR008); the official CLI is a client bex verifies, not one it builds (DO_NOT_DO); pillar 5 sandboxes
- **Expected outcome:** users on the latest Render CLI get Render's behavior from bex, and upstream CLI drift stops going unnoticed
- **Why now:** v2.36.0 shipped three days before filing with no alert; Render dropped the Hobby cap on 2026-11-03, so the cap bex adopted to match Render (w2/m263 D3=A, 2026-11-07) is now a divergence
- **Recommendation in place of an open decision:** the proposal recommended following Render and removing the Hobby cap (D3=A chose "match Render"; ADR008). Keeping it would need a signed ADR018 divergence; the user has not signed one. Idle suspension still stops charges
- **Render parity:** included — sandbox create semantics change on REST/GraphQL/MCP
