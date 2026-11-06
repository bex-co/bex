# w6 · m161 — One tracking-issue helper for every scheduled monitor

**Worker:** worker6 **Goal:** every scheduled monitor opens, updates and closes its tracking issue through one tested helper that cannot open a duplicate on a retried create. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [`scripts/lib/tracking-issue.sh open/close <label> <title> <body-file>`](t001.md) | 45m | — |
| t002 | [Migrate the scheduled monitors to the helper](t002.md) | 60m | t001 |
| t003 | [Self-test with a stubbed gh, including a create whose response is lost](t003.md) | 30m | t001 |
| t004 | [Simplify](t004.md) | 15m | t002, t003 |
| t005 | [Test coverage](t005.md) | 20m | t004 |
| t006 | [Closeout](t006.md) | 10m | t005 |

## Definition of done

- No workflow calls `gh issue create`/`gh issue edit` directly (a grep guard in `scripts/github-actions-validate.sh` or the self-test enforces it).
- The helper's self-test proves a lost create response does not open a duplicate.

## Source + Goal linkage

- **Source:** `/pm-brainstorm w6` 2026-11-06, proposal 3; reviews of `405cbefa6` and `00170e192`.
- **Goal linkage:** ADR008 reliable delivery: monitors must report each incident once and reliably.
- **Expected outcome:** one place to change tracking-issue behaviour; no duplicate incident issues on a flaky API.
- **Why now:** the retry just added makes duplicates possible, and any further change to the sequence would again touch ~80 call sites.
- **Render parity:** omitted — CI/infrastructure only, no REST/GraphQL/MCP/UI surface.
