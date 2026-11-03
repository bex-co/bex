# w6 · m157 — CI checkouts skip the `.pm` tree, so a short job never holds a runner for half an hour

**Worker:** worker6 **Goal:** every CI job that does not read board evidence checks out in seconds, and a new checkout cannot silently fetch `.pm` again. **Status:** todo (t001–t003 done; t004 live measurement next)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Classify every checkout step by what it reads](done/t001.md) — **DONE** | 30m | — |
| t002 | [Make the checkouts that skip `.pm` blobless and sparse](done/t002.md) — **DONE** | 45m | w6/m157/t001 |
| t003 | [Validator rule: a checkout either skips `.pm` or names why it needs it](done/t003.md) — **DONE** | 40m | w6/m157/t002 |
| t004 | [Measure checkout time before and after on real runs](t004.md) | 30m | w6/m157/t002 |
| t005 | [Simplify](t005.md) | 15m | w6/m157/t003, w6/m157/t004 |
| t006 | [Test coverage](t006.md) | 20m | w6/m157/t005 |
| t007 | [Closeout](t007.md) | 10m | w6/m157/t006 |

## Definition of done

- Every `actions/checkout` step in `.github/workflows/` either uses `filter: blob:none` with a non-cone sparse checkout excluding `/.pm/` (re-including only the board paths it reads), or carries a `# full-checkout: <reason>` comment. The secret scan and the markdown format check must see `.pm`.
- `scripts/github-actions-validate.sh` fails a checkout that does neither (red/green test).
- On production runners, a `.pm`-skipping checkout of current main takes under 2 minutes; the 2026-11-03 baseline was 31 min. Run ids are recorded, and the affected workflows stay green on main.

## Source + Goal linkage

- **Source:** promoted from w6/105 (filed from w6/104, 2026-11-03). `Production edge liveness` run 41594756773 held both busy `bex-production` runners in `actions/checkout` for 31 min. `.pm` is 1,448 of 1,553 tracked MB (w6/100). A blobless + sparse clone is 22 MB and takes 47 s.
- **Goal linkage:** ADR008 reliable delivery and ADR083's self-hosted CI. Runner time spent downloading board evidence is taken from deploys, drills and probes.
- **Expected outcome:** checkouts drop from tens of minutes to seconds across all three pools, and deploy runner waits shrink with them.
- **Why now:** it is today's dominant runner hold, and it needs no product decision. w6/100's (A)/(B)/(C) still governs Argo and the repo layout.
- **Render parity:** omitted. CI infrastructure, no REST/GraphQL/MCP/UI surface.

## t001–t003 implementation — 2026-11-03

- **Classification (62 steps):** 54 skip `/.pm/` (blobless, non-cone sparse `/*` + `!/.pm/`). `scripts.yml` `test` also re-includes `/.pm/w*/dev-*/` (dev-env.test.sh reads the per-workstream `ports.env`). Two stay full, each with a `# full-checkout:` reason: `deploy.yml` `secret-scan` (gitleaks must scan every tracked byte) and `docs.yml` `prettier` (formats `.pm` markdown). The three sparse `native-common-image` jobs were already sparse and are unchanged. Exempt and untouched: `native-base-image.yml` and `native-common-image.yml`, whose exact bytes are pinned by w8/m91's D3 custody (`test_d3_transition_pin_binds_the_actual_checked_out_publisher_policy`), until that evidence is re-recorded. Scripts that read `.pm` at runtime: dev-env.test.sh (handled) and native-common qualification (exempt file). Every other `.pm` mention is in comments or messages.
- **Validator:** `github-actions-validate.sh` check 6b fails an `actions/checkout` that neither sets `sparse-checkout` nor carries a `# full-checkout: <reason>` line above it. Commented-out `uses:` lines are ignored, as are the two custody-pinned files and `lego/operator/.github`. There are 4 red/green cases; the shared fixture action moved from checkout to `azure/setup-kubectl` so unrelated cases stay on their own rule.
- **Checks:** validator plus self-test pass, 165 `.github/scripts` tests pass, and `gitops-validate.sh` passes. `test_cli_analytics`/`test_product_analytics` need a local Postgres (environment, unchanged).
