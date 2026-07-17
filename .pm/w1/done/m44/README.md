# w1 · m44 — Team Members parity completion: seats, resend, token accept, security metadata, placement, audit

> Created as `w4/m33` (2026-07-16), moved to `w1/m33` by user action the same day (goal: `implement .pm/w1/m33/README.md`), and **renumbered to m44 at closeout** — `w1/done/m33` (pre-deploy command) already occupied the m33 done slot, and renumbering the new item is the board's established collision fix (cf. w1 README's `022.md` renumber note). Code comments written during implementation reference `w1/m33` — its number while open.

**Worker:** worker1 **Goal:** close every remaining delta between bex's shipped Team Members surface (w4/m12 + w6/m10/m12/m13/m15) and Render's live Team Members section, so the feature is byte-for-byte credible next to `docs/render-artifacts/team-members.graphql` instead of "verbs match, chrome missing". **Status:** done (2026-07-16)

## Tasks (in order)

| id   | title                                                                | est | depends_on         |
| ---- | -------------------------------------------------------------------- | --- | ------------------ |
| t001 | Seat usage read: used/limit on the members surface (REST/GraphQL/MCP) — **DONE** | 40m | —                  |
| t002 | Dashboard seat display: "X of Y seats" + invite-full state — **DONE**            | 30m | t001               |
| t003 | Resend-invite verb across REST/GraphQL/MCP + dashboard action — **DONE**         | 40m | —                  |
| t004 | Token-based invite acceptance (redeem `?invite=<token>`) — **DONE**              | 60m | —                  |
| t005 | Member security metadata: `mfaEnabled` on MemberView + dashboard badge — **DONE** | 40m | —                  |
| t006 | Mount team management under `/workspace/settings` — **DONE**                     | 40m | t002, t003, t005   |
| t007 | Audit events for member operations — **DONE**                                    | 40m | —                  |
| t008 | Render parity check across REST/GraphQL/MCP/UI — **DONE**                        | 30m | t004, t006, t007   |
| t009 | Simplify pass over the milestone's changes — **DONE** (residuals filed as `w1/026`) | 30m | t008               |
| t010 | Test coverage for the shipped behavior — **DONE**                                | 40m | t008               |
| t011 | Closeout — **DONE**                                                              | 20m | t009, t010         |

## Definition of done

On a dev-N stack: the Team panel renders under `/workspace/settings`, shows "X of Y seats" sourced from the API (not inferred client-side), each member row shows a 2FA indicator, a pending invite can be resent (fresh email observed in Mailpit, expiry refreshed) without revoke+re-invite, an invite redeemed via its emailed `?invite=<token>` link joins the workspace even when the recipient signed up under a different email, and every member mutation (invite/resend/change-role/remove/revoke) produces an `audit_events` row visible in the audit panel. REST, GraphQL, and MCP expose the same new fields/verbs with the same semantics.

**Verified live on dev-1, 2026-07-16** (screenshot `.playwright-mcp/m33-team-workspace-settings.png`): Team panel between the details card and danger zone on `/workspace/settings`; "1 of 1 seats" on Hobby → "N seats used" after the Scale upgrade; invite to `invitee@bex.test` delivered to Mailpit; Resend delivered a second mail with refreshed expiry and unchanged token; the emailed link redeemed by `newcomer@bex.test` (≠ invited address) who joined as DEVELOPER with the FGA tuple written; dev1's row shows the 2FA badge after a real TOTP enrollment (second-factor login exercised); `members.Invite`/`ResendInvite`/`AcceptInvite` rows visible in the audit panel with invite targets, email, and `roleTo` detail. Suites: backend `go test ./...` green incl. real-Postgres integration for `RefreshInvite`/`AcceptInviteByToken` (migration 0040 applied), `make lint-backend` 0 issues, dashboard 1250 tests + typecheck + lint green, cross-surface parity locked by `TestThreeSurfaceParity_SeatUsageAndMFA`.

## Source + Goal linkage

- **Source:** Team Members investigation 2026-07-16 — live side-by-side of `dashboard.render.com/w/tea-…/settings` vs bex `/workspace/settings` on dev-1, plus a full code map of `lego/backend/internal/members/` and `dashboard/src/features/team/`. Render contract evidence: `docs/render-artifacts/team-members.graphql` (usage.users used/limit, member otpEnabled/active, pendingInvites) and `docs/render-artifacts/dashboard-walk/workspace.md`.
- **Goal linkage:** Render parity (docs/ADR018-render-parity.md — members row) + the multi-tenant IAM mission (w4/MISSION-IAM.md, docs/ADR024-members.md).
- **Expected outcome:** the Team Members surface stops losing on the details a Render user notices in the first minute — seat count, resend, invite links that work cross-email, 2FA visibility, and the section living where Render puts it — and member mutations become auditable.
- **Why now:** the core verbs shipped and hardened across w4/m12→w6/m15; what remains is exactly this enumerated gap list (full board scan 2026-07-16 confirmed no open milestone covers any item). t006 deliberately revisits w5/m36's "placement is an IA choice, not a gap" ruling per user direction 2026-07-16, now that `/workspace/settings` exists and already cross-links the team panel via the `?plan=change` CTA. Render parity closing task included: every implementation task touches REST/GraphQL/MCP and/or the dashboard.
- **Deferred:** workspace ownership transfer → `.pm/FUTURE-MAYBE.md` (recorded with trigger, same date). Avatar upload and a Kratos `name` trait stay known divergences (docs/ADR024-members.md § Known divergence) — out of scope here.
