# w4 · m115 — Database Insights is blind: Active processes + Top queries render 34 `<insufficient privilege>` cells

**Worker:** worker4 **Goal:** a database owner opening Insights sees their processes and top queries — or an honest explanation — never 34 raw Postgres mask strings. **Status:** done 2026-09-20 (live re-probe of the deployed fix deferred to the next QA pass — the grant is declarative and lands on the next reconcile of every existing database)

## Tasks (in order)

| id   | title                                                                                          | est  | depends_on |
| ---- | ---------------------------------------------------------------------------------------------- | ---- | ---------- |
| t001 | Give the insights path visibility into tenant query activity (grant, not masking)              | 1h30 | —          | — **DONE**
| t002 | Dashboard never renders raw `<insufficient privilege>` rows                                    | 45m  | t001       | — **DONE**
| t003 | Render parity + docs                                                                           | 20m  | t002       | — **DONE**
| t004 | Simplify                                                                                       | 20m  | t003       | — **DONE**
| t005 | Test coverage                                                                                  | 45m  | t004       | — **DONE**
| t006 | Closeout                                                                                       | 10m  | t005       | — **DONE**

## Definition of done

Each bullet is a click the next person can repeat on production and watch succeed.

- **Processes are visible.** On a throwaway database with recent SQL-console activity, Insights → Active processes shows real query text for the owner's own sessions (today: every Query cell reads `<insufficient privilege>`).
- **Top queries are visible.** Top queries shows real query text with calls/timing (today: all 25 rows masked).
- **No raw mask strings.** No `<insufficient privilege>` cell ever renders; if any row is legitimately hidden, the section says why (today: 34 masked cells, zero explanation).
- **Least privilege preserved.** The fix grants introspection visibility only — the insights path stays read-only (`default_transaction_read_only` + statement timeout intact).

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` pass on `https://dashboard.bex.co`, 2026-09-19 (w4-targeted run, `muse.env` credentials). Fixture: Postgres `qa-20260919-p14-db` (`dpg-dan32k92dbts73fi28o0`, deleted after the pass) with a live table + SQL-console round trip. Insights section: Active processes 9 rows × (User blank, Query `<insufficient privilege>`), Top queries 25 rows × Query `<insufficient privilege>` — 34 masked cells, unchanged after Refresh. Storage + Table scans + Parameters on the same page render fine.
- **Goal linkage:** managed-Postgres introspection ([docs/ADR009-postgresql-management.md](../../../docs/ADR009-postgresql-management.md)) — Insights is the flagship "live introspection" surface and two of its six panels are unusable.
- **Expected outcome:** the insights connection can read the activity it is shown to display, and the dashboard degrades honestly when it cannot.
- **Why now:** the mechanism is nailed in code (`runInsight` dials the tenant DB with the unprivileged app-user URI, so Postgres itself masks every other role's query text) — the fix is a provisioning grant plus a UI guard, not an investigation.
- **Explicitly out:** this pass's re-confirmed clean areas (provisioning, TLS, SQL-console CRUD + guards, Metrics charts, Recovery empty state, CIDR validation, disk-autoscaling persistence) — not re-filed.

## Outcome (2026-09-20)

- **t001 — the grant.** `managedRoles` (`lego/operator/internal/controller/database_controller.go`) now always projects the database **owner** into CNPG `spec.managed.roles` as a `pg_monitor` member. Declarative, so it converges on **existing** databases at the next reconcile — no backfill pass. It deliberately carries no `passwordSecret` (CNPG only NULLs a password on roles it creates; the owner always already exists, and naming the `-app` Secret would make the projection depend on one the recovery bootstrap does not always produce). Least privilege is preserved and asserted, not assumed: `TestInsightConnectionKeepsItsReadOnlyRails` pins `default_transaction_read_only=on` plus the statement timeout on the insights connection.
- **t002 — honest degradation, backend-typed.** `<insufficient privilege>` is now folded into a typed `masked` flag with an empty `query` at the mapping site (`processViews`/`topQueryViews` in `insights.go`), so REST, GraphQL (`masked` on `DatabaseProcess`/`DatabaseTopQuery`), MCP and the dashboard all get the same signal instead of the raw literal. The Insights panel renders a "Hidden" cell plus one per-section explanation, keeps unmasked rows and every real statistic visible, and **also** treats the bare literal as masked so an older bex-api cannot leak it.
- **t003 — parity + docs.** ADR009 gains an "Insights visibility: the owner holds `pg_monitor`" section (decision, the two deliberate details, and the inRoles-revocation consequence); ADR018 row 150 records the grant and the additive `masked` field, noting Render has no equivalent process/top-query API so this is a bex extension.
- **t005 — coverage.** Backend: `TestMaskedQueryTextNeverReachesASurface` (masked → flag + empty query, real columns/statistics survive, an *embedded* placeholder stays real SQL), `TestMaskedIsCarriedByEveryInsightSurface` (JSON tags + both GraphQL fields), `TestInsightConnectionKeepsItsReadOnlyRails`. Operator: `TestCnpgClusterSpecManagedRoles` extended — owner always present, `inRoles: [pg_monitor]`, no `passwordSecret`, even with zero tenant users. Dashboard: `insights-panel.test.tsx`, 4 cases (fully masked, partial, old-API literal, nothing masked); **verified failing against the pre-fix panel by stash-run** (3 of 4 fail).
- **Green:** `lego/backend` `go test ./...` all packages, `lego/operator` `make test` all packages, dashboard `yarn test` 3518 tests + `yarn typecheck` + `yarn lint`, and `golangci-lint run` 0 issues in both operator and backend. `make lint` from `lego/operator` fails in the **`lego/cli`** module only — `lego/cli/go.mod` declares `go 1.27.0` and the local toolchain is `go1.26.5`, a pre-existing environment mismatch unrelated to this milestone (no `lego/cli` file was touched).
- **Not done — t006's live re-probe.** Confirming the 34 masked cells are gone needs the fix deployed and a throwaway production database; deferred to the next QA pass, the same disposition m113/m112/m111 carry. The grant needs no migration step: the next reconcile of each existing Database applies it.
- **Adjacent, checked and not a gap:** `bootstrap.recovery` skips `postInitSQL`, so a restored cluster never runs `CREATE EXTENSION pg_stat_statements` at bootstrap — but ADR009 §"Legacy query-insights convergence (w8/m16)" already covers it: every reconcile projects the `pg_stat_statements.track` managed-extension switch, which makes CNPG create the extension in each connectable database. No follow-up filed.
