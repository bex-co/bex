# w6 · m157 — CI checkouts skip the `.pm` tree, so a short job never holds a runner for half an hour

**Worker:** worker6 **Goal:** every CI job that does not read board evidence checks out in seconds, and a new checkout cannot silently fetch `.pm` again. **Status:** done (2026-11-03)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Classify every checkout step by what it reads](done/t001.md) — **DONE** | 30m | — |
| t002 | [Make the checkouts that skip `.pm` blobless and sparse](done/t002.md) — **DONE** | 45m | w6/m157/t001 |
| t003 | [Validator rule: a checkout either skips `.pm` or names why it needs it](done/t003.md) — **DONE** | 40m | w6/m157/t002 |
| t004 | [Measure checkout time before and after on real runs](done/t004.md) — **DONE** | 30m | w6/m157/t002 |
| t005 | [Simplify](done/t005.md) — **DONE** | 15m | w6/m157/t003, w6/m157/t004 |
| t006 | [Test coverage](done/t006.md) — **DONE** | 20m | w6/m157/t005 |
| t007 | [Closeout](done/t007.md) — **DONE** | 10m | w6/m157/t006 |

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

## t004 result and closeout — 2026-11-03 (DoD met)

| run | pool / runner | checkout step | baseline |
| --- | --- | --- | --- |
| ci-red-streak 41602503636 (success) | bex-production `45e29ad2967e` | 99 s | 31 min (Production edge liveness 41594756773, 07:12–07:43) |
| scripts 41609029657 on main `210b8976c` (success) | bex-ci | 113 s | — |
| scripts 41603283943 | bex-ci `239d6e52dba9` | 32 s | — |

- Run 41609029657 proves the `scripts.yml` re-include of `/.pm/w*/dev-*/` works: dev-env.test.sh passed against the sparse tree.
- Run 41603283943 failed only its SHA-pin inventory step. Its head `2f0798250713` is a PR branch, not on main, carrying older action SHAs; main passes the same check. Unrelated to this milestone.
- **t005 Simplify:** the change is mechanical and self-reviewed. The one shared fixture change (checkout → setup-kubectl in the validator tests) keeps each case on its own rule.
- **t006 Test coverage:** 4 validator cases (full tree fails, sparse passes, named full passes, an empty reason fails), plus the existing suites.
- **Not changed:** `native-base-image.yml` / `native-common-image.yml` keep full checkouts until w8/m91's D3 custody pins are re-recorded. Argo's own checkout is w6/100's decision.
