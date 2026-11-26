# w6 · m654 — Render templates that ask for values deploy on bex once the values are given

**Worker:** worker6 **Goal:** twenty `render-examples` Blueprints that ask for values (`sync: false`) are deployed on production in `bex-canary` through the plan's `requiredManualEnvVars` and the approval's `manualEnvVars`, as Render's Deploy to Render form asks for them, and each ends in a recorded outcome. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Choose 20 templates that ask for values, and the value each key gets](t001.md) | 45m | — |
| t002 | [Supply `sync: false` values in `scripts/render-examples-live.sh`](t002.md) | 1h | t001 |
| t003 | [Run the 20 templates on production in `bex-canary` and triage every failure](t003.md) | 2h30m | t002, w6/185 |
| t004 | [Record the asked-for-values coverage in ADR018 and ADR049](t004.md) | 30m | t003 |
| t005 | [Simplify](t005.md) | 20m | t004 |
| t006 | [Test coverage](t006.md) | 45m | t004 |
| t007 | [Closeout](t007.md) | 10m | t005, t006 |

## Definition of done

1. On a production pin the result page names, 20 templates chosen in t001 were deployed in `bex-canary` by `scripts/render-examples-live.sh`, each with its `sync: false` values passed through the approval's `manualEnvVars`.
2. Each one ends in exactly one of these outcomes:
   - live, with its check passing;
   - failed, with the failure filed as a w6 inbox note or matched to an open board item;
   - failed, with evidence that Render fails the same way;
   - needs a real key: the process refuses the placeholder at start, with the line that says so.
3. At least one template went from plan to live through supplied values with no other change.
4. `docs/render-artifacts/render-examples-live.md` has a section for these runs: the rule for values, the table and the outcomes. No value that is a real credential appears in it or in any commit.
5. After the run, `bex-canary` holds none of the resources the run created.
6. ADR018's Blueprint row and ADR049 § Manual env vars cite the section.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6`, 2026-11-26 (the fifth run that day), item 2, filed by `/pm them to w6`.
- **Goal linkage:** ADR008 § Render parity first. Render's Deploy to Render asks for each `sync: false` value before it creates the services. bex's counterpart is the plan's `requiredManualEnvVars`, which the approval must answer (`BLUEPRINT_MANUAL_ENV_VARS_MISSING`, `BLUEPRINT_MANUAL_ENV_VAR_UNKNOWN`; ADR049 § Manual env vars). No real template has been deployed through that path end to end. The row is ADR018 [`deployment-sources-iac.md`](../../../docs/ADR018/deployment-sources-iac.md), "Blueprint / `render.yaml` IaC".
- **Expected outcome:** the first measure of the half of Render's templates that m644 and w6/m653 cannot reach, and one board item per failure class.
- **Why now:**
  - w6/m644's first batch found a bex-side failure in 4 of 20 templates, and none of the templates that ask for values has ever been run.
  - The harness exists.
  - t003 waits on [w6/185](../185.md) for the same GitHub budget reason as w6/m653.
- **Corpus facts (2026-11-26):** `bash scripts/render-examples-corpus.sh <dir>` fetched 185 files. Leaving out the 44 with a workflow or previews, about 66 have at least one `sync: false` key; t001 recounts.
  - The most common keys are `ANTHROPIC_API_KEY` (19 files), `RENDER_API_KEY` (19) and `OPENAI_API_KEY` (18).
  - Files ask for between one key and more than five: 11 ask for one, 16 for two, 18 for three, and 18 for five or more.
- **Render parity task omitted:** this milestone measures, extends a script and records. It changes no REST, GraphQL, MCP or dashboard surface. Each gap it finds is filed as its own item, which carries its own parity check.
- **Assumptions, not user decisions:**
  - Third-party keys get obvious placeholders, and the check is build, deploy and start, not the feature behind the key.
  - Templates that name paid plans are deployed as written in `bex-canary` and deleted after each run.
  - A `RENDER_API_KEY` template may call `api.render.com` by name. t003 records what it does with a bex key and files nothing on that alone.
