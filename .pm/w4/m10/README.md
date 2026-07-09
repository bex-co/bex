# w4 · m10 — Audit log: every Core write recorded with caller identity

**Worker:** worker4 **Goal:** Every authenticated write verb (and every authz denial) leaves one record — caller (`api.IdentityFrom`), verb, resource, outcome, timestamp — captured at one interception point in the feature kernel, stored in the control-plane store, readable by workspace admins. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                    | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Audit hook at the one interception point (`internal/core/base.go` + authz middleware): verb, caller, resource, allow/deny    | 30m | —          |
| t002 | `audit_events` table in `internal/store` + retention cap (time- or count-based, documented)                                  | 25m | t001       |
| t003 | Read surface: REST `GET /v1/audit-events` + GraphQL query, admin-scoped (`can_manage`), filterable by resource/time          | 30m | t002       |
| t004 | Acceptance: every write verb emits exactly one event; a 403 denial is recorded; values/secrets never appear in events        | 25m | t003       |
| t007 | Render parity — audit surface consistency vs Render's audit-logs page/API (retrofit 2026-07-09)                              | 20m | t004       |
| t005 | Simplify — `/simplify` over the code this milestone changed                                                                  | 20m | t007       |
| t006 | Test coverage — meaningful tests for the behavior this milestone shipped                                                     | 30m | t007       |
| t008 | Closeout — DoD met → move milestone to `done/` (retrofit 2026-07-09)                                                         | 10m | t006       |

## Definition of done

Suspend a service → one audit event carrying the caller's identity; a cross-tenant 403 → one denial event; env-var writes are recorded **without** values; the read surface pages newest-first and refuses non-admins. Store-less mode (no `BEX_CP_DB_URI`) degrades to no-op recording + 503 reads (omitted, not faked).

## Source + Goal linkage

- **Source:** `/pm-brainstorm tasks for w4` 2026-07-08; Render's workspace Audit Log page (parity); `GOAL.md` item 7 (security review), which will demand this evidence.
- **Goal linkage:** multi-tenant security (roadmap #5/#7); pillar 1 — Render workspaces ship audit logs.
- **Expected outcome:** "who deleted that service?" becomes answerable; the security review has evidence to review.
- **Why now:** the write-verb surface roughly doubles with w2/m4–m6 just queued — one interception point added now covers them for free; retrofitted later, every verb needs revisiting. The store it needs shipped in w1/m2.
