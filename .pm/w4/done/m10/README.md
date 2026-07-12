# w4 · m10 — Audit log: every Core write recorded with caller identity

**Worker:** worker4 **Goal:** Every authenticated write verb (and every authz denial) leaves one record — caller (`core.IdentityFrom`), verb, resource, outcome, timestamp — captured at one interception point in the feature kernel, stored in the control-plane store, readable by workspace admins. **Status:** done — verified via unit + integration tests (a real throwaway Postgres for the store layer, `httptest` over the real composition root for REST/GraphQL); `go build`/`go vet`/`go test ./...`/`golangci-lint` all green in `lego/backend`. Live mock-cluster smoke (t004's suggested extra check) was **not** run this session — no cluster available in this environment; recommended before a prod deploy.

## Tasks (in order)

| id   | title                                                                                                                    | est | depends_on | |
| ---- | -------------------------------------------------------------------------------------------------------------------------- | --- | ---------- | --- |
| t001 | Audit hook at the one interception point (`internal/core/base.go` + authz middleware): verb, caller, resource, allow/deny    | 30m | —          | — **DONE** |
| t002 | `audit_events` table in `internal/store` + retention cap (time- or count-based, documented)                                  | 25m | t001       | — **DONE** |
| t003 | Read surface: REST in Render's audit-logs shape (owner/workspace-scoped path, cursor paging) + GraphQL query, admin-scoped (`can_manage`) | 30m | t002       | — **DONE** |
| t004 | Acceptance: every write verb emits exactly one event; a 403 denial is recorded; values/secrets never appear in events        | 25m | t003       | — **DONE** |
| t007 | Render parity — audit surface consistency vs Render's audit-logs page/API (retrofit 2026-07-09)                              | 20m | t004       | — **DONE** |
| t005 | Simplify — `/simplify` over the code this milestone changed                                                                  | 20m | t007       | — **DONE** |
| t006 | Test coverage — meaningful tests for the behavior this milestone shipped                                                     | 30m | t007       | — **DONE** |
| t008 | Closeout — DoD met → move milestone to `done/` (retrofit 2026-07-09)                                                         | 10m | t006       | — **DONE** |

## Definition of done

Suspend a service → one audit event carrying the caller's identity; a cross-tenant 403 → one denial event; env-var writes are recorded **without** values; the read surface pages newest-first and refuses non-admins. Store-less mode (no `BEX_CP_DB_URI`) degrades to no-op recording + 503 reads (omitted, not faked).

## What shipped (2026-07-11)

- **Hook** (`lego/backend/internal/core/audit.go` + `base.go`): `Base.Authorize`/`AuthorizeOn` both funnel through a shared `authorizeAndAudit`, recording success/denial for the write-tier relations only (`can_operate`/`can_create`/`can_manage_keys`/`can_manage`). The verb name is resolved from the call stack (`callerVerb`), not hand-threaded through ~80 call sites — no verb can forget to be recorded. `Base.emit` bounds the sink call to a 2s timeout; a sink error is logged and swallowed, never fails the verb.
- **Store** (`lego/backend/internal/store/audit.go` + `migrations/0008_audit_events.*.sql`): `audit_events` table (no FK to `tenants` — a purged tenant's trail and a denied cross-tenant guess must both survive), keyset (newest-first) pagination. `*store.PGStore` structurally satisfies `core.AuditSink` — wired directly onto `core.Base.Audit` in `cmd/api/main.go`. Retention: `internal/audit.Service`'s daily sweep, `BEX_AUDIT_RETENTION_DAYS` (default 90), same loop shape as usage's w8/m4 compaction.
- **Read surface** (`lego/backend/internal/audit`): `GET /v1/owners/{ownerId}/audit-logs` (Render's path/query vocabulary) + GraphQL `auditLogs(ownerId, startTime, endTime, cursor, limit)`, both delegating to one `Service.List` (`can_manage`-scoped, 503 store-less). Field-shape divergence from Render (exact JSON schema wasn't publicly resolvable) is documented in `docs/ADR006-bex-api.md` and `docs/ADR018-render-parity.md`.
- **Tests**: `internal/core/audit_test.go` (hook unit tests, fake sink), `internal/store/store_pg_test.go`'s `assertAuditEvents` (real Postgres), `internal/audit/service_test.go` (List/purge/retention-loop unit tests), `internal/api/audit_surface_test.go` (REST/GraphQL parity + auth matrix), `internal/api/server_test.go`'s `TestAuditCoversEveryWriteVerbExactlyOnce` + `TestAuditNeverCarriesSecretValues` (coverage + hygiene, reusing `TestAuthzGuardsEveryVerb`'s inventory so the two can't drift).
- **Docs**: `docs/ADR006-bex-api.md` § Audit log, `docs/ADR012-auth.md` consequences note, `docs/ADR018-render-parity.md` row flipped to ◐ (REST/GraphQL shipped; shape divergence documented), `CLAUDE.md` + `.env.example`/`.env.template` for `BEX_AUDIT_RETENTION_DAYS`.
- **Not done this session**: live mock-cluster/prod smoke (t004's optional extra check) — no cluster available in this environment. MCP/dashboard UI exposure — out of scope per t003.

## Source + Goal linkage

- **Source:** `/pm-brainstorm tasks for w4` 2026-07-08; Render's workspace Audit Log page (parity); `GOAL.md` item 7 (security review), which will demand this evidence.
- **Goal linkage:** multi-tenant security (roadmap #5/#7); pillar 1 — Render workspaces ship audit logs.
- **Expected outcome:** "who deleted that service?" becomes answerable; the security review has evidence to review.
- **Why now:** _2026-07-11 update — the window this line predicted is closing, not upcoming._ Five write-verb milestones have shipped un-audited since this was written: w2/m4 (service delete), w2/m5 (deploy objects), w2/m7 (keyvalue), w1/m16 (env groups + secret files), w4/m12 (members/invites) — the retrofit cost is already accruing, and the rebuilt prod (real multi-tenant cluster, live CP store, live OpenBao env-vars) makes the audit-evidence gap a live security-review item (GOAL item 7). t004's inventory reuse is the retrofit vehicle; sequence m10 ahead of further verb milestones. The store it needs shipped in w1/m2 and is live + restored in prod.
