# w10 · m2 — Docs & code truth sweep

**Worker:** worker10 **Goal:** the stale documentation claims and twice-filed-never-done small items that fed this round's false gap findings get corrected — the parity evidence base (ADR006, render-artifacts) tells the truth again, `make lint` goes clean, and the one real small code gap (`maxShutdownDelaySeconds` in `bex.yml`) closes. **Status:** DONE (2026-07-15 — all tasks shipped: ADR006/key-value.md stale claims corrected with evidence pointers; ADR006's MCP table backfilled with every real `apps`/`postgres`/`keyvalue` MCP tool (far beyond the originally-scoped 5, once the sweep found the table's drift ran deeper); `maxShutdownDelaySeconds` threaded through the `bex.yml` Blueprint parser onto `CreateRequest`, reusing `Create`'s existing validation (no duplicate range check) — round-trip + out-of-range tests added; `envVarKey` comment corrected (already wired, verified against `resolveRef` + existing tests) and `gqlStr`/`gqlStrPtr` graduated into `internal/gqlutil` (one definition each, ~9 call sites migrated); pg-sni-proxy/build_test's 16 tracked lint findings plus 15 more that had accumulated since (activator, controller) all fixed properly — zero `//nolint`s; `/simplify` review (4-angle) applied 2 real fixes (graduated `gqlStrPtr` too; aligned `wakeApp`'s context-logger with the operator's established `logf` idiom) and skipped 2 out-of-scope/conflicting findings; `make lint` zero findings both modules, full backend + operator suites green)

## Tasks (in order)

| id   | title                                                                  | est | depends_on                         |
| ---- | ----------------------------------------------------------------------- | --- | ---------------------------------- |
| t001 | Fix stale ADR006 claims (rollback :521, deferred-HA/observability :285) + `render-artifacts/key-value.md:27` — **DONE** | 30m | —                                  |
| t002 | Backfill the 5 missing `set_*` MCP tools into ADR006's reference table — **DONE** | 30m | —                                  |
| t003 | Thread `maxShutdownDelaySeconds` through the `bex.yml` parser — **DONE** | 45m | —                                  |
| t004 | Fix `envVarKey` stale comment + graduate `gqlStr` into `internal/gqlutil` — **DONE** | 30m | —                                  |
| t005 | Clear the 16 pre-existing `pg-sni-proxy`/`build_test` lint findings — **DONE** | 45m | —                                  |
| t006 | Render parity (narrow: t003's bex.yml field) — **DONE**                | 15m | t003                               |
| t007 | Simplify — **DONE**                                                     | 30m | t001, t002, t004, t005, t006       |
| t008 | Test coverage — **DONE**                                                | 45m | t006                               |
| t009 | Closeout — **DONE**                                                     | 15m | t008                               |

## Definition of done

Every listed stale claim is corrected with evidence pointers (grep for the quoted phrases returns nothing); ADR006's MCP table lists all real tools; a `bex.yml` with `maxShutdownDelaySeconds` round-trips into the App spec; `internal/gqlutil` owns a single `gqlStr`; `make lint` (both modules) reports zero findings.

**Met.** Grep for all three quoted stale phrases returns nothing (`docs/ADR006-bex-api.md`, `docs/render-artifacts/key-value.md`). ADR006's MCP reference table (`docs/ADR006-bex-api.md`) now lists every tool actually registered in `apps/mcp.go`, `postgres/mcp.go`, and `keyvalue/mcp.go` — not just the 5 originally named. `TestBlueprintMaxShutdownDelayRoundTripsAndValidates` (`lego/backend/internal/apps/maxshutdowndelay_test.go`) proves a `bex.yml` carrying the field lands on `App.spec.maxShutdownDelaySeconds` and that an out-of-range value fails with the same named `core.ErrBadRequest` as REST. `internal/gqlutil.Str` (+ `StrPtr`) is the sole definition; `grep -rn "^func gqlStr\b"` under `lego/backend/internal/` returns nothing. `make lint` from `lego/operator` reports `0 issues` for both the operator and backend modules; `go test ./...` is green in both `lego/operator` and `lego/backend`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 12, 2026-07-15 — this round's miners returned three false gaps traced to stale docs (ADR006:521 "Not yet: rollback" vs shipped w2/m10; :285 "Still deferred" vs shipped w1/m22 + w2/m25; key-value.md:27 vs shipped w2/m16); plus the twice-filed MCP-table backfill (ADR006:352 + w4/done/m21's follow-up), ADR006:160's recorded bex.yml drift, `apps/deploy.go:325`'s stale comment, w6/done/m24's `gqlStr` flag, and the recurring 16-finding lint debt (w8/done/m12, w2/done/m36). Grouped sub-hour chores per the w1/m23 / w1/m30 pattern. Coordinate with (don't duplicate) `w5/013`'s ADR018 stale-marker sweep — different files, same disease.
- **Goal linkage:** the parity program's evidence base — every future `/pm-brainstorm` round and the w7/m30 conformance suite reason from these docs.
- **Expected outcome:** doc-mining stops returning false positives; lint debt stops recurring in closeout caveats.
- **Why now:** the cost is now measurable — this very round burned verification effort on three phantom gaps. Render parity closing task included narrowly for t003 (a Blueprint-surface field); the remainder is docs/lint with no surface change.
