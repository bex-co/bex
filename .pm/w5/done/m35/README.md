# w5 · m35 — Dashboard dead-ends: Postgres parameter-overrides editor + workspace resource caps

**Worker:** worker5 **Goal:** Two shipped backend capabilities the dashboard fetches but never lets a human use become usable: the Insights panel's Parameter Overrides section gains the editor its already-wired `saveParameters` hook was built for, and workspaces with resource caps see used-vs-limit before they hit the 4xx. **Status:** done

## Tasks (in order)

| id   | title                                                                                | est | depends_on | status   |
| ---- | ------------------------------------------------------------------------------------ | --- | ---------- | -------- |
| t001 | Parameter-overrides editor (add/edit/remove + save via the existing `saveParameters`) | 60m | —          | — **DONE** |
| t002 | Resource-caps display: "N of M" services/Postgres/Key Value where limits are set      | 45m | —          | — **DONE** |
| t003 | Render parity                                                                        | 20m | t001, t002 | — **DONE** |
| t004 | Simplify                                                                             | 15m | t003       | — **DONE** |
| t005 | Test coverage                                                                        | 40m | t003       | — **DONE** |
| t006 | Closeout                                                                             | 15m | t005       | — **DONE** |

## Definition of done

A user changes a Postgres parameter override from the Insights panel and sees it applied (live check against a dev-N stack); a workspace with caps configured shows used-vs-limit for services/Postgres/Key Value (unlimited ⇒ hidden); no dead-wired mutation hooks remain in `features/databases`; en + zh locales complete.

## Resolution

The Insights panel now consumes its existing parameter mutation through a replace-style add/edit/remove editor. It validates incomplete, duplicate, and platform-managed rows before submit; preserves rejected drafts with the backend message inline; refetches immediately after a successful mutation; and polls the live `pg_settings` view until the applied value/source converges. The Usage page now queries the selected workspace's three cap counters, hides `limit=0` resource kinds (and the whole card when all are unlimited), warns at 80%, and polls independently of create/delete feature areas. Both surfaces have English and Chinese copy.

The `dev-5` live proof ran with all three caps configured to 10. Authenticated GraphQL returned `0/10` for services, Postgres, and Key Value. A fresh managed Postgres became Available; saving `work_mem=16MB` through the shipped API projected to CNPG and the live read returned `work_mem=16384 kB`, source `configuration file`; saving an empty replacement removed it from the live view. The test database and temporary mock-cluster plumbing were removed afterward.

The parity ledger now records Render's current arbitrary-string-map `parameterOverrides` create/update contract and bex's dedicated REST/GraphQL/MCP extensions, plus Render's documented Hobby/Free limits and bex's unified cap-read visibility superset. No additional drift note was needed. A behavior-preserving simplify review found no further safe reductions after the editor/component and cap/hook splits. Final verification passed dashboard lint, all 202 test files / 1,184 tests, the production client and SSR build, GraphQL schema generation, Markdown Prettier, and `git diff --check`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 14, 2026-07-15 — dashboard capability-diff miner, both verified: (a) `dashboard/src/features/databases/hooks/use-database-insights.ts:61-96` builds and exports `saveParameters(...)` over the `SetDatabaseParameterOverrides` mutation (`databases.graphql:331`) with **zero consumers**; `insights-panel.tsx:214-244` renders Parameter Overrides read-only while the backend PUT shipped in w2/m25 (`lego/backend/internal/postgres/rest.go:359`). (b) `GET /v1/owners/{id}/limits` + GraphQL `workspaceLimits {services|postgres|keyValues {used, limit}}` (`lego/backend/internal/workspaces/graphql.go:46-61`, w7/m9) appear only in the generated schema (`dashboard/src/graphql/definitions.ts:1524,1952-1962`) — no operation, no component; users discover `BEX_MAX_SERVICES` by hitting the cap error.
- **Goal linkage:** dashboard completeness (w5's charter) — Render's dashboard exposes both concepts (Postgres config tuning; plan limits).
- **Expected outcome:** two already-paid-for backend features become usable by humans; the stranded WIP hook is either consumed or gone.
- **Why now:** the editor hook is literally wired and exported — stranded work-in-progress, the purest "polish existing" item this round; caps UX prevents a support-question class as multi-user workspaces grow (w4/m12 invites shipped).
- **Render parity:** included — UI surface change; t003 compares Render's parameter-override and limits presentation.
