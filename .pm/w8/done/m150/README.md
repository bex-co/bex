# w8 · m150 — Blueprint create reads a public repo even while GitHub's API stumbles

**Worker:** worker8 **Goal:** creating or syncing a Blueprint from a public GitHub repo succeeds through transient GitHub API failures: idempotent GitHub reads retry, and a public repo's ref and file are read over git transport and GitHub's raw CDN when the API still fails. **Status:** done (2026-11-12: t001–t006 shipped 2026-11-10; t007's production replay planned a Blueprint from the public repository, fixture deleted)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record the 2026-11-08 failure's cause where production logs still hold it — **DONE** | 20m | — |
| [t002](done/t002.md) | Idempotent GitHub GETs retry a 5xx or reset within the request budget — **DONE** | 40m | — |
| [t003](done/t003.md) | A public repo's Blueprint ref and file fall back to git transport and the raw CDN — **DONE** | 60m | t002 |
| [t004](done/t004.md) | Render parity — **DONE** | 15m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004 |
| t007 | Closeout — **DONE** (live Blueprint create on production) | 20m | t006 |

## Definition of done

1. A unit test where `api.github.com` answers 502, resets the stream or times out for a public repo still resolves the branch and reads `render.yaml` (through the retry, or through `ls-remote` + `raw.githubusercontent.com` at the resolved commit), and create plans. Private-repo, 404 and access-denied behavior is unchanged.
2. The fallback is logged with the API failure it replaced.
3. On production, `bex ea blueprints create --repo https://github.com/bex-co/bex --branch main --path examples/hello-node/render.yaml … --existing-resources create_new` in a workspace with no GitHub connection returns `planned`. The fixture is deleted afterwards.

## Source + Goal linkage

- **Source:** promoted from [w8/240](../done/240.md) on 2026-11-10 (`/qa-find-bugs-cli` w8 loop sweeps `18cb`/`8c72`/`bf43`, 2026-11-08). The finding, evidence and analysis are kept there.
- **Goal linkage:** Render parity first (ADR008). Blueprints (ADR049) are Render's IaC entry point, and Render reads a public repo's `render.yaml` without the customer seeing GitHub's API health.
- **Expected outcome:** a transient GitHub API error no longer blocks Blueprint creation from a public repo. Bex retries, or reads over the transports that kept working on 2026-11-08.
- **Why now:** Blueprint creation was impossible for over 40 minutes on 2026-11-08 while git transport worked, and the customer could neither see nor fix the cause.
- **Render parity included:** the change is behind REST/GraphQL/MCP Blueprint create, sync and validate and the dashboard's New Blueprint.

## Blocked — 2026-11-10

Everything but t007 is done and shipped. t007 replays the Definition of done on production, which needs (1) the scheduled 19:00 UTC deploy (production deploys run at 19:00/00:00/05:00 UTC since `4630a48a7`) and (2) a signed-in run. `scripts/qa-login.sh` is rejected by Kratos (rc 1 on 2026-11-10, as on 2026-11-08), so the QA password must be fixed or the user must re-authorize their own CLI login with run-owned `bex-canary` fixtures, as they did for the 2026-11-08 sweep.
