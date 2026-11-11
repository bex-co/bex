# w21 · m324 — A deploy's log page shows only that deploy's build output

**Worker:** worker21 **Goal:** when two deploys overlap in time, each deploy's page shows its own build and pre-deploy lines and none of the other's. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Backend: a deploy-scoped build and pre-deploy log read — **DONE** | 60m | — |
| t002 | GraphQL: `logs(deployId:)` and the regenerated definitions — **DONE** | 20m | t001 |
| t003 | Dashboard: the deploy page scopes its build and pre-deploy legs — **DONE** | 30m | t002 |
| t004 | Render parity — **DONE** | 20m | t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 30m | t004, t005 |
| t007 | Closeout — **DONE** | 10m | t006 |

## Definition of done

- With two overlapping deploys (a hook called twice one second apart, the second with a ref that cannot resolve), the superseded deploy's page shows none of the second deploy's lines, and the failed deploy's page shows none of the first's "Deploy canceled"; each page still shows its own full story.
- The Service Logs tab, REST `GET /v1/logs` and MCP return both deploys' lines over the same window.
- The new tests fail if the scoping is removed.
- The production repro (journey 3 of `/qa-find-bugs` pass 42) runs after the deploy; it was not run in this session.

## Source + Goal linkage

- **Source:** inbox note [w21/044](../044.md), promoted: it was filed as one hour but spans the logs query model, a GraphQL argument, build-pod attribution and the dashboard. Found by live `/qa-find-bugs` pass 42, 2026-11-10.
- **Goal linkage:** observability and Render parity (each Render deploy's logs are its own; ADR010).
- **Expected outcome:** a user debugging a canceled or failed deploy reads that deploy's lines, not another deploy's error.
- **Why now:** a CI that fires the deploy hook twice, or two quick pushes, makes this routine, and the log pane is where a user looks first.
- **Render parity task:** included — it touches the GraphQL `logs` surface and the deploy page; the decision is recorded in ADR010.
