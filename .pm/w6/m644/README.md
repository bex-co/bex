# w6 · m644 — Render's own templates deploy unchanged on bex and serve, not just validate

**Worker:** worker6 **Goal:** twenty of Render's own `render-examples` Blueprints, deployed unchanged on production in `bex-canary`, each either go live and pass the check their README or Render guide gives, or end in a filed item (or proof that Render fails the same way); one command reruns any template. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Choose 20 templates that cover every runtime and service type, each with its documented check](t001.md) | 45m | — |
| t002 | [Write `scripts/render-examples-live.sh`: deploy one template unchanged, check it, delete it](t002.md) | 1h | t001 |
| t003 | [Run the 20 templates on production in `bex-canary` and triage every failure](t003.md) | 2h | t002 |
| t004 | [Record the live coverage in ADR018 and ADR049](t004.md) | 30m | t003 |
| t005 | [Simplify](t005.md) | 20m | t004 |
| t006 | [Test coverage](t006.md) | 45m | t004 |
| t007 | [Closeout](t007.md) | 10m | t005, t006 |

## Definition of done

1. On a production pin named in `docs/render-artifacts/render-examples-live.md`, all 20 templates chosen in t001 were deployed unchanged in `bex-canary` by `scripts/render-examples-live.sh`.
2. Each template ends in exactly one of these outcomes:
   - live, with its documented check passing;
   - failed, with the failure filed as a w6 inbox note or matched to an open board item;
   - failed, with evidence that Render fails the same way.
3. `docs/render-artifacts/render-examples-live.md` holds the result table (template, runtime, types, outcome, failing stage, item). No raw log or other evidence file is committed.
4. `bash scripts/render-examples-live.sh <repo>` reruns any one template from a clean checkout, given only `BEX_API_URL` and a `bex-canary` API key.
5. After the run, `bex-canary` holds none of the resources the run created.
6. ADR018's Blueprint row and ADR049's corpus-guard paragraph cite the result page.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6`, 2026-11-26, the single item it proposed, filed by `/pm new milestone`. That run compared render.com/docs/docker, /docker-secrets, /health-checks, /disks and /deploys with bex clause by clause and found every clause already matched. The previous run, on /free, found one stale claim (`c2295b80de`). The proposal recorded the candidates it rejected and why.
- **Goal linkage:** AGENTS.md's top rule, "Render parity is the top product principle" (ADR008 § Render parity first): every customer-facing behavior must match render.com. A Render template that deploys with one click on Render must deploy unchanged on bex. The row this closes is ADR018 [`deployment-sources-iac.md`](../../../docs/ADR018/deployment-sources-iac.md), "Blueprint / `render.yaml` IaC". Its template coverage today is validation only: w5/m143, plus the weekly `TestRenderExamplesCorpus` job from w33/001, which accepts 166 of 181 files, and every refusal is one Render makes too or a `DO_NOT_DO` non-goal.
- **Expected outcome:** the first repeatable measure of how many of Render's own templates actually run on bex, and one board item per failure class. The same harness can widen the sample later.
- **Why now:**
  - Reading docs no longer finds gaps; real deploys still do. Each time a `render-examples` repo was deployed for real, it surfaced a major bug that validation passed: `render-examples/clickhouse` → w6/m148 (ports) and w6/m149 (container privileges); `render-examples/phoenix_hello` → w32/037 (502 forever) and w8/373 (Elixir/OTP pairs); Ruby and Node repos → w9/133, w8/399, w8/401 and w8/307.
  - w6 had no ready item when this was filed: `scripts/loopx-guard.py board w6` listed `ready: []`, and m640 waits on a production pin.
- **Corpus facts (2026-11-26):** `bash scripts/render-examples-corpus.sh <dir>` fetched 185 `render.yaml` files.
  - 97 of them have no `sync: false` variable (so no third-party secret is needed), no workflow and no preview, so they can be deployed unchanged.
  - Across those 97, runtimes appear as docker 27, node 17, python 7, image 6, static 5, ruby 3 and go 2. Service types appear as web 56, pserv 14, worker 5, redis 3 and cron 1. 21 files declare databases and 23 declare a disk.
- **Render parity task omitted:** this milestone measures and records. It adds a script and a docs page, and changes no REST, GraphQL, MCP or dashboard surface. Each gap it finds is filed as its own item, which carries its own parity check.
- **Assumption, not a user decision:** some chosen templates name paid plans. They are deployed as written in `bex-canary` (a first-party workspace) and deleted after each run, so the cost is a few instance-hours. If the user prefers otherwise, t001 can restrict the sample to free plans, at the cost of the disk and private-service templates.
