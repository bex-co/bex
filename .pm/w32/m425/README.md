# w32 · m425 — A production capture of Render's default environment variables, by service type, runtime and phase

**Worker:** worker32 **Goal:** ADR018's ✅ for default environment variables rests on a committed production capture of every variable Render documents, and the acceptance checks still owed for w8/272 and w8/202 are closed. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Probe fixtures and capture plan | 30m | — |
| t002 | Deploy and capture on production | 75m | t001 |
| t003 | Record the capture as `docs/render-artifacts/default-env-live.md` | 30m | t002 |
| t004 | Fix the small mismatches, file the rest | 45m | t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 30m | t004 |
| t007 | Closeout | 20m | t005, t006 |

## Definition of done

1. `docs/render-artifacts/default-env-live.md` records, for every variable render.com/docs/environment-variables lists, its observed presence and value at build and at runtime for each service type (web, private, worker, cron, static build) and source (native Node, Python, Ruby, Go, Rust, Elixir; Dockerfile build args; prebuilt image), with the production pin, the capture date and Render's fetch date. Each cell is ✅, ✖ with a fix commit or filed id, or — with a reason.
2. w8/272's owed acceptance (a Phoenix release with no clustering configuration boots with a long `node()`; `RELEASE_DISTRIBUTION=sname` set by the service keeps a short name) and w8/202's pending hosted acceptance are walked on production and recorded in ADR018.
3. Rows already proven on production reuse that evidence (w8/273, w8/274).
4. The ADR018 default-variables row links the capture.
5. Fixtures are deleted (by-id `GET` 404).

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for Render parity of render.com/docs/environment-variables, 2026-11-16, item 3; re-verified before filing. Correction from the proposal: w8/273 and w8/274 already passed on production, so this milestone reuses them.
- **Evidence:** no `docs/render-artifacts` capture exists for the default-variables row, while the env-var, secret-file and env-group rows each have one; w8/272 moved to `done/` with its production acceptance "Still owed" (`.pm/w8/done/272.md:48`); ADR018's `PORT` row says of w8/202 "Operator deployment and hosted acceptance remain pending".
- **Goal linkage:** ADR008 Render parity first; ADR018's ✅ backed by evidence rather than by unit, kind and local-build checks alone.
- **Expected outcome:** a committed matrix that any later default-variable change can be checked against, and two owed acceptance checks closed.
- **Why now:** default-variable changes shipped between 2026-11-11 and 2026-11-14 (w8/256, w8/272, w8/273, w8/274, w4/386, w21/076), and w32/m424 adds three more. Run it after w32/m424's code is on production so the capture sees those three; if m424 is still waiting on its production pin, mark those cells pending rather than waiting.
- **Render parity:** omitted: a verification milestone whose fixes are runtime-environment only. If t004 changes a REST, GraphQL, MCP or dashboard surface, add the Render parity task with `/pm add-task`.
