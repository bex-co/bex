# w6 · m9 — Security hygiene chores

**Worker:** worker6 **Goal:** clear the `w6/m6` audit's remaining sub-hour backlog in one shippable chunk, including an urgent pre-existing test break. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                  | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Enforce PKCE for all `authorization_code` clients (Hydra `require_pkce`) — from `003`                     | 30m | —          |
| t002 | Constant-time comparison hardening: webhook either-key OR, internal tenant-API bearer `subtle.ConstantTimeCompare` — from `004` | 30m | —          |
| t003 | Collapse GitHub upstream 5xx error body before forwarding to bex-api callers — from `005`                 | 20m | —          |
| t004 | CRD validation markers for `Repo`/`Branch`/`RootDir` in `lego/types` + `make manifests generate` — from `007` | 45m | —          |
| t005 | Fix nil-pointer panic in `SetAutoscaling`/`DisableAutoscaling` GraphQL resolvers (red tests, pre-existing) — from `008` | 30m | —          |
| t006 | Render parity — verify t004/t005's user-facing validation-error shape and autoscaling mutation behavior vs render.com | 30m | t004,t005  |
| t007 | Simplify — `/simplify` over the code this milestone changed                                                | 15m | t006       |
| t008 | Test coverage — meaningful tests for each fix's real behavior (PKCE rejection, timing-safe compare, error collapse, CRD rejection, autoscaling mutation) | 30m | t006       |
| t009 | Closeout — verify each fix's end state, then move the milestone to `done/`                                 | 10m | t007,t008  |

## Definition of done

All five fixes land as described; `TestGraphQLSetAutoscalingMutation`/`TestGraphQLDisableAutoscalingMutation` pass; `cd lego/backend && go test ./...` is green.

## Source + Goal linkage

- **Source:** `w6/003.md`, `w6/004.md`, `w6/005.md`, `w6/007.md`, `w6/008.md` — all filed from `w6/m6`'s security review (2026-07-11/12), grouped via `/pm-brainstorm` 2026-07-12 per the sizing rule (each individually sub-hour); notes moved to `done/`.
- **Goal linkage:** `GOAL.md` #7, clearing the `w6/m6` audit's low-priority backlog before it goes stale.
- **Expected outcome:** PKCE mandated for every authorization_code client, two timing side-channels closed, no upstream error-text leakage, CRD-level input validation for hand-applied Apps, and the pre-existing autoscaling-mutation test break fixed.
- **Why now:** each item is sub-hour and homeless (the sizing rule keeps them out of their own milestones); t005 in particular is an urgent pre-existing test break the audit flagged "fix before the next `/ship` that touches `internal/apps`."
- **Render parity closing task: included** — t004/t005 touch validation error shape and a GraphQL mutation, both user-facing.
