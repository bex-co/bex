# w9 · m84 — Role-aware dashboard controls: disable-with-reason instead of always-fail

**Worker:** worker9 **Goal:** a contributor sees role-gated controls disabled with a legible reason instead of an editable field that 403s on save; the dashboard gains viewer-capability plumbing the feature components read. **Status:** done

## Tasks (in order)

| id   | title                                                                                          | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Backend: `viewer`/`capabilities` GraphQL projection (caller role + effective verbs)            | 45m | —          — **DONE** |
| t002 | Dashboard: thread a `capabilities` context/hook the feature components read                    | 45m | t001       — **DONE** |
| t003 | Disable-with-reason on the four m68 contributor-boundary controls                              | 45m | t002       — **DONE** |
| t004 | Extend to the other standing boundaries the dashboard ignores                                  | 45m | t003       — **DONE** |
| t005 | Render parity (closing)                                                                         | 20m | t004       — **DONE** |
| t006 | Simplify (closing)                                                                             | 20m | t005       — **DONE** |
| t007 | Test coverage (closing)                                                                        | 30m | t005       — **DONE** |
| t008 | Closeout (closing)                                                                             | 10m | t007       — **DONE** |

## Definition of done

A contributor viewing an affected control (set pre-deploy command, set cron `command`, create a one-off job, deploy with `imageUrl`/`commitId`, set Postgres statement-logging params) sees it **disabled with a role-reason tooltip**, not an editable field that 403s on save; an admin sees it enabled. The same disable-with-reason treatment is applied to the other standing role boundaries the dashboard ignores (`can_view_sensitive` reveals, `can_manage_billing`, `can_manage` members). The pattern is disable-with-reason, never hide. Tests assert gating per role (contributor vs admin) for each affected control.

## Source + Goal linkage

- **Source:** `.pm/w1/047.md` (promoted 2026-08-17 via `/pm-brainstorm` "what to do for w9"); the follow-up `w1/m68` t008 Render-parity task recorded that m68's `can_operate`→`can_create` verb move left the dashboard rendering controls that always fail. `docs/ADR024-members.md` § "The contributor boundary: lifecycle, not code" is the server-side rule this mirrors in the UI.
- **Goal linkage:** Render parity + first-party dashboard quality (the human-facing surface, `dashboard/CLAUDE.md`); makes the workspace role model legible instead of discoverable-by-failing.
- **Expected outcome:** contributors stop hitting always-fail controls; the role model teaches itself through disabled-with-reason affordances; admins are unaffected.
- **Why now:** latent today (most workspaces are single-member admin) but becomes visible the first time a real workspace has non-admin members; the backend already exposes `members[].role` (`lego/backend/internal/members/graphql.go`) so the projection is a small addition, and the dashboard is warm from the m60–m83 sweep. **Render parity included** — it changes REST/GraphQL surface (viewer projection) and the dashboard UI, so cross-surface consistency must be checked.
