# w6 · m37 — Protected-environment guard for Postgres/KeyValue delete + suspend

**Worker:** worker6 **Goal:** a Postgres database or Key Value instance that belongs to a `protectedStatus=protected` Environment can no longer be deleted or suspended without the same typed confirm-phrase gate Apps already enforce. **Status:** done

## Tasks (in order)

| id   | title                                                                                                       | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Store: `GetEnvironmentProtectedStatus(ctx, environmentID)` + label-based protected-status resolution for Database/KeyValue CRs — **DONE** | 45m | —          |
| t002 | `postgres` package: `protection.go` (`requireUnprotected`/`ProtectedConfirmation`) wired into `Delete` + suspend — **DONE** | 1h  | t001       |
| t003 | `keyvalue` package: same guard wired into `Delete` + suspend — **DONE**                                                   | 1h  | t001       |
| t004 | REST: thread `?confirm=` through Postgres/KeyValue delete + suspend routes (incl. `/v1/databases` alias) — **DONE**       | 45m | t002, t003 |
| t005 | GraphQL: `confirm` arg on `deleteDatabase`/`suspendDatabase`/`deleteKeyValue`/`suspendKeyValue` — **DONE**                | 30m | t002, t003 |
| t006 | MCP: `Confirm` field on `suspend_postgres`/`suspend_keyvalue` (no MCP delete tool exists for either — deliberate, matches Render's own MCP server; do not add one) — **DONE** | 30m | t002, t003 |
| t007 | Dashboard: generalize `ProtectedConfirmationDialog` and wire it into the Database/KeyValue row-actions delete + suspend flows, catching the protected-environment 400 and prompting the server-issued phrase — **DONE** | 1h  | t004, t005 |
| t008 | Render parity: verify confirm-phrase semantics consistent across REST/GraphQL/MCP/UI vs. the existing Apps guard — **DONE** | 30m | t006, t007 |
| t009 | Simplify — **DONE**                                                                                                        | 30m | t008       |
| t010 | Test coverage — **DONE**                                                                                                   | 1h  | t008       |
| t011 | Closeout — **DONE**                                                                                                        | 15m | t009, t010 |

## Definition of done

A Postgres database (or Key Value instance) that is a member of a `protectedStatus=protected` Environment rejects `Delete` and `Suspend` with a named 400 unless the caller echoes back the exact `ProtectedConfirmation`-style phrase — proven end to end over REST, GraphQL, and MCP (suspend only) — exactly mirroring how `apps.Service` already guards Apps (`lego/backend/internal/apps/protection.go`). The dashboard's database/key-value delete and suspend dialogs catch that 400 and prompt the required phrase, reusing the existing `ProtectedConfirmationDialog` pattern services already ship. An unprotected or environment-less Database/KeyValue behaves byte-identically to today (opt-in, no regression). A regression test proves a protected member is blocked without the phrase and unblocked with it, for both resource types.

## Source + Goal linkage

- **Source:** `/pm feature parity check and fix around delete a psql database against render` (2026-07-16), code investigation across `lego/backend/internal/{apps,postgres,keyvalue}` and `.pm/w6/done/m19,m20`. `w6/m19`'s own closeout text names this exact gap ("a 'protected' environment should protect its database too") but only implemented the guard for Apps; `w6/m20` added Database/KeyValue Environment *membership* (the `core.LabelEnvironment` CR label) without the destructive-verb guard riding on top of it. `docs/ADR032-environments.md:43` and `docs/render-artifacts/protected-environments.md:15` (Render's captured contract: protection's restricted-action set explicitly covers "deleting resources" for datastores) both confirm this is a real, currently-unclosed divergence — not a documented deliberate non-goal, so it isn't blocked by `.pm/DO_NOT_DO.md`.
- **Goal linkage:** Render parity + platform safety hardening (same charter as `w6/m19`): protected Environments are meant to block destructive verbs uniformly across every resource type they can contain, not just Apps. A safety guard that silently doesn't apply to two of the three member resource types is a trust-eroding gap, not a cosmetic one.
- **Expected outcome:** deleting or suspending a Postgres/KeyValue instance that's a member of a protected Environment requires the same explicit, typed confirmation Apps already require — closing a real data-loss guard hole before it's hit in production.
- **Why now:** `w6/m19` flagged this as a known follow-up at the moment it shipped the App-only guard; it has sat open since 2026-07-14 while `w6/m20` (same day) added exactly the membership data this milestone needs to read. The longer it stays open, the more Postgres/KeyValue instances can accumulate inside protected Environments believing they're covered when they aren't. Render parity included (t008) — this is a tenant-facing safety verb on all applicable surfaces.
