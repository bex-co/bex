# w8 · m10 — Env vars: `generateValue` + cursor pagination

**Worker:** worker8 **Goal:** Close the env-vars row's two documented omissions — Render's `generateValue: true` (server-minted random value) and list pagination — identically across REST, GraphQL, MCP, and the dashboard. **Status:** todo

## Tasks (in order)

| id   | title                                                 | est | depends_on |
| ---- | ----------------------------------------------------- | --- | ---------- |
| t001 | Core: `generateValue` on the env-var write verb       | 45m | —          |
| t002 | Thread `generateValue` through REST · GraphQL · MCP   | 40m | t001       |
| t003 | Cursor pagination on env-var list endpoints           | 45m | —          |
| t004 | Dashboard: Generate-value affordance + paged list     | 40m | t002, t003 |
| t005 | Render parity                                         | 30m | t004       |
| t006 | Simplify                                              | 30m | t005       |
| t007 | Test coverage                                         | 45m | t005       |
| t008 | Closeout                                              | 15m | t007       |

## Definition of done

`generateValue: true` on an env-var write mints a cryptographically random value server-side, with identical semantics and error shapes on REST, GraphQL, and MCP (value stored in OpenBao like any env var, readable back per the existing reveal flow). The env-var list endpoints accept `cursor`/`limit` with the established `{object, cursor}` envelope. The ADR006 divergence note ("omits pagination + generateValue") is gone.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker to work on until feature parity` 2026-07-14 (item 5); `docs/ADR018-render-parity.md` §Environment & config env-vars row → `docs/ADR006-bex-api.md` divergence note.
- **Goal linkage:** Render parity core (env vars are the highest-traffic config surface); prerequisite for w1/m35's bex.yml `generateValue` acceptance.
- **Expected outcome:** real-world Render clients and blueprints using `generateValue` work unmodified; large env sets page instead of unbounded lists.
- **Why now:** w8's queue is empty (m1–m9 done); w1/m35 depends on t001's core verb. Placed under w8 per the w8/m8 capacity-placement precedent. Render parity task included — all-surface change.
