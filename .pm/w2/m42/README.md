# w2 · m42 — Deploy commit author timestamp (`commit.createdAt`)

**Worker:** worker2 **Goal:** deploys expose Render's nested `commit.createdAt` from the resolved Git commit's real author timestamp — captured once at deploy-open, never inferred from the deploy row — across REST/GraphQL/MCP and the dashboard. **Status:** todo

## Tasks (in order)

| id   | title                                                        | est | depends_on |
| ---- | ------------------------------------------------------------ | --- | ---------- |
| t001 | Author-timestamp capture at deploy-open + store migration    | 45m | —          |
| t002 | `commit.createdAt` on REST/MCP + GraphQL                     | 30m | t001       |
| t003 | Dashboard display; omit-when-unavailable                     | 30m | t002       |
| t004 | Render parity                                                | 30m | t003       |
| t005 | Simplify                                                     | 30m | t004       |
| t006 | Test coverage                                                | 45m | t004       |
| t007 | Closeout                                                     | 15m | t006       |

## Definition of done

A Git-backed deploy's REST/GraphQL/MCP representations carry `commit.createdAt` equal to the commit's author timestamp (verified against the actual repo commit); when resolution is unavailable the field is omitted — never fabricated from deploy time; the dashboard deploy detail shows it; `docs/render-artifacts/deploy-detail-page.md:26`'s residual is closed.

## Source + Goal linkage

- **Source:** promoted from inbox `w2/011` (filed by `w2/m38`'s 2026-07-15 Render recheck), `/pm-brainstorm` round 12.
- **Goal linkage:** Render deploy-object parity (extends w2/m38's lifecycle depth + w9/m1's deploy page).
- **Expected outcome:** the deploy object's commit block matches Render field-for-field.
- **Why now:** w2/m38's commit-resolution path is freshly shipped — the capture point exists; the milestone is small while the code is warm. Render parity closing task included — REST/GraphQL/MCP/UI change.
