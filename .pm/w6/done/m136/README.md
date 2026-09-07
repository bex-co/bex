# w6 · m136 — Fail-closed capability contract: tri-state outcomes, resource-action preconditions, freshness (ADR087)

**Worker:** worker6 **Goal:** any client (mobile first) can ask "what can this caller see and do in this workspace / on this resource" and get an answer that distinguishes **allowed / denied / unavailable** with bounded reason codes — never a permissive guess, and never a checker failure dressed up as "your role forbids this". **Status:** done — 2026-09-07. Tri-state grants + bounded reasons + fresh evaluation live on viewerCapabilities across GraphQL/REST/MCP; per-resource action projections (serverActions/deployActions/databaseActions/keyValueActions) computed by the execute paths' own predicates; ADR087 matrix pinned red-proven; /simplify 3-agent pass applied (AuthorizeFresh reuse, shared ProtectionPrecondition, lazy preconditions, single workspace resolution per request); full backend suite (61 pkgs) + lint-backend + whole-program deadcode green.

## Tasks (in order)

| id   | title                                                                     | est | depends_on |
| ---- | ------------------------------------------------------------------------- | --- | ---------- |
| t001 | Pin the ADR087 action matrix with backend regression tests — **DONE**      | 45m | —          |
| t002 | Tri-state capability outcomes + bounded reason codes on viewerCapabilities — **DONE** | 45m | t001 |
| t003 | Explicit workspace + resource context binding — **DONE** | 45m | t002 |
| t004 | Per-resource action precondition projection — **DONE** | 60m | t003 |
| t005 | Freshness semantics: cached-vs-fresh evaluation + recovery path — **DONE** | 30m | t002 |
| t006 | Render parity: capability contract aligned across REST/GraphQL/MCP — **DONE** | 30m | t004, t005 |
| t007 | Simplify — **DONE** | 30m | t006 |
| t008 | Test coverage — **DONE** | 45m | t006 |
| t009 | Closeout — **DONE** | 15m | t008 |

## Definition of done

The shared capability surface (`viewerCapabilities` and its counterparts wherever the contract is exposed) reports, per grant: allowed / denied / unavailable, with a bounded non-sensitive reason code distinguishing missing OAuth scope, insufficient permission, and authorization-service failure. Resource-scoped action decisions bind the exact kind/id in the supplied workspace and reject out-of-context targets; no silent default-workspace substitution. A documented fresh-evaluation path exists for recovery after an access change, and the 30s positive-cache boundary is documented. A checker outage yields `unavailable` (fail closed), never a confident denial reason. The ADR087 action matrix (rollback=`can_create`, bare trigger/cancel/lifecycle/cron/session reads=`can_operate`, logs=`can_view_logs`, `databaseProcesses`=sensitive scope) is pinned by a regression test proven red on drift. Existing boolean callers keep working; roles and native OAuth scopes are unchanged.

## Source + Goal linkage

- **Source:** [docs/ADR087-mobile-role-views.md](../../../docs/ADR087-mobile-role-views.md) §Required contract work (items 1–5) + §Delivery sequence step 1; materialized 2026-09-07 per user direction (`/pm for w6`). ADR claims verified against the checkout the same day (members/service.go:310, members/graphql.go:128, core/base.go Can→false collapse, deploys/service.go:765/657/414, agentsessions/service.go:705/859/922, api/scope_matrix_overrides.go:78, authz/authz.go 30s positive cache).
- **Goal linkage:** ADR048 mobile supervision + ADR012/ADR024 authorization pillars — the mobile hide/show product (m138) is unbuildable without a fail-closed capability answer; today `Core.Can` collapses checker errors into `false` and `viewerCapabilities` exposes only coarse booleans.
- **Expected outcome:** clients can distinguish "you may not" from "we couldn't check", and can ask about a specific resource action's preconditions without probing mutations.
- **Why now:** hard prerequisite for m138 (mobile role views); the contract gaps are the ADR's launch blockers ("reason-specific UX, reliable role-change detection, and complete resource-action gating require the corresponding contract gaps to be closed"). Render parity task included — the contract is exposed on user-facing API surfaces and must stay aligned across REST/GraphQL/MCP (ADR087 requirement 5).
