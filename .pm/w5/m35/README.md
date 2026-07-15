# w5 · m35 — Dashboard dead-ends: Postgres parameter-overrides editor + workspace resource caps

**Worker:** worker5 **Goal:** Two shipped backend capabilities the dashboard fetches but never lets a human use become usable: the Insights panel's Parameter Overrides section gains the editor its already-wired `saveParameters` hook was built for, and workspaces with resource caps see used-vs-limit before they hit the 4xx. **Status:** todo

## Tasks (in order)

| id   | title                                                                                | est | depends_on |
| ---- | -------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Parameter-overrides editor (add/edit/remove + save via the existing `saveParameters`) | 60m | —          |
| t002 | Resource-caps display: "N of M" services/Postgres/Key Value where limits are set      | 45m | —          |
| t003 | Render parity                                                                          | 20m | t001, t002 |
| t004 | Simplify                                                                               | 15m | t003       |
| t005 | Test coverage                                                                          | 40m | t003       |
| t006 | Closeout                                                                               | 15m | t005       |

## Definition of done

A user changes a Postgres parameter override from the Insights panel and sees it applied (live check against a dev-N stack); a workspace with caps configured shows used-vs-limit for services/Postgres/Key Value (unlimited ⇒ hidden); no dead-wired mutation hooks remain in `features/databases`; en + zh locales complete.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 14, 2026-07-15 — dashboard capability-diff miner, both verified: (a) `dashboard/src/features/databases/hooks/use-database-insights.ts:61-96` builds and exports `saveParameters(...)` over the `SetDatabaseParameterOverrides` mutation (`databases.graphql:331`) with **zero consumers**; `insights-panel.tsx:214-244` renders Parameter Overrides read-only while the backend PUT shipped in w2/m25 (`lego/backend/internal/postgres/rest.go:359`). (b) `GET /v1/owners/{id}/limits` + GraphQL `workspaceLimits {services|postgres|keyValues {used, limit}}` (`lego/backend/internal/workspaces/graphql.go:46-61`, w7/m9) appear only in the generated schema (`dashboard/src/graphql/definitions.ts:1524,1952-1962`) — no operation, no component; users discover `BEX_MAX_SERVICES` by hitting the cap error.
- **Goal linkage:** dashboard completeness (w5's charter) — Render's dashboard exposes both concepts (Postgres config tuning; plan limits).
- **Expected outcome:** two already-paid-for backend features become usable by humans; the stranded WIP hook is either consumed or gone.
- **Why now:** the editor hook is literally wired and exported — stranded work-in-progress, the purest "polish existing" item this round; caps UX prevents a support-question class as multi-user workspaces grow (w4/m12 invites shipped).
- **Render parity:** included — UI surface change; t003 compares Render's parameter-override and limits presentation.
