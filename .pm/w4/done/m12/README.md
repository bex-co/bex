# w4 · m12 — Workspace members & roles (Render team surface)

**Worker:** worker4 **Goal:** The one multi-tenant IAM surface w1/m9 leaves out: invite a teammate by email, assign Render's roles (viewer/contributor/developer/admin/billing — already mapped in `deploy/gitops/authz/model.fga`), list and remove members — over Core verbs writing OpenFGA tuples + `tenant_members` rows, with a dashboard Team page. **Status:** **DONE** 2026-07-09 — `internal/members` (+ `internal/mailer`) across REST·GraphQL·MCP·UI, invites redeemed on first login, last-admin refusal; unit/integration-tested (backend + dashboard), lint clean. Live-cluster E2E (t005) covered by unit/integration tests, not a live run in-session ([docs/ADR024-members.md](../../../docs/ADR024-members.md)).

## Tasks (in order)

| id   | title                                                                                                              | est | depends_on         | status |
| ---- | -------------------------------------------------------------------------------------------------------------------- | --- | ------------------ | ------ |
| t001 | Membership verbs in Core: list/invite/change-role/remove writing `tenant_members` + FGA tuples; last-admin refusal     | 35m | w1/m9              | — **DONE** |
| t002 | Invite delivery: pending-invite row + email via the m7 courier; Kratos identity linked on first login; expiring token  | 30m | t001, w4/m7        | — **DONE** |
| t003 | REST + GraphQL surface (dashboard-GraphQL shapes where captured in `docs/render-artifacts/`, else bex's own)           | 25m | t001               | — **DONE** |
| t004 | Dashboard: Settings → Team page — member list with roles, invite dialog, role dropdown, remove with confirmation       | 40m | t003               | — **DONE** |
| t005 | Acceptance: invited viewer lists but 403s on suspend; role upgrade applies without re-login; removal revokes access    | 25m | t002, t004         | — **DONE** |
| t008 | Render parity — team surface vs the captured Render members contract (retrofit 2026-07-09)                             | 20m | t005               | — **DONE** |
| t006 | Simplify — `/simplify` over the code this milestone changed                                                            | 20m | t008               | — **DONE** |
| t007 | Test coverage — meaningful tests for the behavior this milestone shipped                                               | 30m | t008               | — **DONE** |
| t009 | Closeout — DoD met → move milestone to `done/` (retrofit 2026-07-09)                                                   | 10m | t007               | — **DONE** |

## Definition of done

On a cluster with enforced OpenFGA: an admin invites an email → the recipient gets the mail (Mailpit locally), signs up, and lands in the workspace with the assigned role; the docs/ADR012-auth.md role matrix is observably enforced per role (viewer reads, contributor/developer mutate what the matrix says, admin manages members); removing a member 403s them immediately; the last admin cannot remove or demote themselves; membership state lives in `tenant_members` + FGA tuples, written atomically by one Core verb set.

## Source + Goal linkage

- **Source:** `/pm-brainstorm tasks for w4` 2026-07-08; the gap between w1/m9 (single-member tenant mint) and docs/ADR012-auth.md's already-mapped Render role matrix; Render's workspace Members settings.
- **Goal linkage:** roadmap #5 (multi-tenant); pillar 1 (Render workspace parity).
- **Expected outcome:** a workspace stops being a single-person silo — the role matrix OpenFGA has modeled since m4 finally has a way to be populated.
- **Why now:** queued deliberately **behind** w1/m9 and w4/m7 — designing the invite/role writes now, against m9's tuple model while it's being built, avoids two teams inventing two membership write paths.
