# Workspace members & roles (w4/m12)

The one multi-tenant IAM surface [w1/m9](ADR003-control-plane.md) left out: invite a teammate by email, assign one of Render's five roles, list members, change a role, and remove a member — over Core verbs that write `tenant_members` rows and OpenFGA `workspace:<id>` tuples together, with a dashboard **Settings → Team** page. It turns the role matrix OpenFGA has modelled since m4 ([ADR012-auth.md](ADR012-auth.md), `deploy/gitops/authz/model.fga`) from a static model into a populated one: a workspace stops being a single-person silo.

## The shape

One feature package, three thin adapters, Render-consistent — the same rule the rest of bex-api follows ([ADR006-bex-api.md](ADR006-bex-api.md)):

- **`lego/backend/internal/members`** — the Core verbs (`service.go`): `List`, `SeatUsage`, `Invite`, `ChangeRole`, `Remove`, `ListInvites`, `ResendInvite`, `RevokeInvite`, and `AcceptInvite` (`SeatUsage`/`ResendInvite`/`AcceptInvite` are w1/m33 — see § Parity completion). Every workspace-scoped verb targets an **explicit workspace id** and gates on _that_ workspace (`AuthorizeOn(rel, workspace:<id>)` — the mutations acting on one member/invite use `AuthorizeOnTarget`, so their audit rows carry a `member:`/`invite:` target), like the workspace-lifecycle verbs — managing members is an admin acting **on** a workspace it administers, not on its own default workspace. `AcceptInvite` is the exception by design: the token is the capability and the caller is _becoming_ a member, so its gate is only "an authenticated, onboarded caller" (`Authorize(can_view)` on the caller's own workspace).
- **`rest.go` / `graphql.go` / `mcp.go`** — the fragments. REST under `/v1/workspaces/{id}/members` + `/invites` + `/seat-usage`, plus the unscoped `POST /v1/invites/accept` (bex extensions — Render manages members only via its dashboard GraphQL); GraphQL `workspaceMembers` (carrying `workspaceSeatUsage` in the same document) / `workspaceInvites` + the six mutations (the dashboard Team page's client); MCP nine tools (an agent seats teammates the same way a human does).
- **`lego/backend/internal/store/members.go`** — the write side of membership: role update / remove on `tenant_members`, the `tenant_invites` lifecycle (incl. `RefreshInvite`), and the shared redemption core behind `AcceptInvitesForEmail` (login email-match) and `AcceptInviteByToken` (direct link). Reads (`ListTenantMembers`/`CountTenantMembers`) live in `store/workspaces.go`.
- **`lego/backend/internal/mailer`** — a provider-agnostic SMTP sender for the invite email; wired from `BEX_SMTP_ADDR`/`BEX_SMTP_FROM` (the same relay the Kratos courier uses — SendGrid in prod, Mailpit locally).

## Roles

The five Render roles, verified live (`docs/render-artifacts/team-members.graphql`) and already the FGA relations in `model.fga`:

| role | can, per the matrix ([ADR012-auth.md](ADR012-auth.md)) |
| --- | --- |
| `viewer` | read-only resource lists/details/metrics; **not** logs or sensitive fields |
| `contributor` | + logs, restart/suspend/resume; **not** create/delete or sensitive fields |
| `developer` | + create/delete, connection strings, env vars, API keys; **no** org settings |
| `admin` | full resources **and** org settings — members, billing, protected envs |
| `billing` | billing only (+ non-sensitive names) |

Roles are **UPPERCASE on the wire** (Render's enum: `ADMIN`, `DEVELOPER`, …) and lowercase as the stored role / FGA relation; adapters convert at the view boundary. A change of role is a Revoke of the old relation tuple then a Grant of the new one; the `tenant_members.role` column is the source of truth, the tuple a best-effort follow-up (the resolver re-drives a missing grant on login).

### Roles are plan-gated (w6/m12)

Not every role is assignable on every plan — Render's live capture (`docs/render-artifacts/team-members.graphql`, `RESEARCH-workspaces.md` finding 5) shows the role picker itself narrows by plan: **Hobby** has no invites at all (single member, always `admin`); **Pro** offers `admin`/`developer` only; **Scale**/**Enterprise** add `contributor`/`viewer`/`billing`. bex encodes this as `PlanLimits.AllowedRoles` (`lego/backend/internal/store/plans.go`'s `LimitsFor`, alongside the member/service caps) plus a `RoleAllowedOnPlan(plan, role) bool` predicate, and enforces it in two places:

- `members.Invite` / `members.ChangeRole` reject a role outside `RoleAllowedOnPlan(tenant.Plan, role)` with a `core.ErrBadRequest` naming the plan and its allowed roles (`guardPlanRole`).
- `workspaces.Service.ChangePlan`'s downgrade guard refuses a plan change while any member holds a role the **target** plan's `AllowedRoles` no longer offers, naming the blocking member(s) — a downgrade must shed the disallowed roles first (`ChangeRole`/`Remove` them) before the plan can flip.

No migration is needed for pre-existing data: roles shipped after plans (w4/m12 came after w6/m1), so no workspace could have assigned an out-of-plan role before this guard existed.

## Invites: pending row → redeem on login

An invite is addressed to an **email**, which has no OpenFGA subject yet — the recipient may not have signed up. So an invite is a `tenant_invites` row (id, email, role, token, `expires_at`, `accepted_at`), not a membership. It is redeemed on the recipient's **first authenticated login**:

1. The auth gate reads the caller's verified email from their Kratos identity traits (`internal/api/auth.go` `whoami`) and passes it to onboarding.
2. `tenantService.acceptInvites` (`internal/api/tenancy.go`) calls `AcceptInvitesForEmail`, which — in one transaction — turns every outstanding, unexpired invite for that email into a `tenant_members` row at its role and marks the invite accepted, then writes the matching OpenFGA role tuple.
3. Onboarding's **personal-tenant** resolution is owner-keyed (`CreateTenantWithMember`'s `owner_identity_id` upsert), **not** membership-keyed, so an invited membership never masquerades as the caller's own workspace — the admin grant stays pinned to the workspace the caller actually owns, never the one they were invited to as a viewer.

Invites expire (Render's 7 days). Delivery is best-effort: a flaky relay is logged, not surfaced — the invite row exists and redeems by email match regardless, and an admin can **resend** (`ResendInvite`, w1/m33: fresh mail, expiry pushed forward, id and token unchanged so the original link stays live; a lapsed-but-unaccepted invite is revived). Set `BEX_DASHBOARD_URL` to give the email a sign-up deep link.

**The emailed link is directly redeemable (w1/m33).** The `?invite=<token>` in the invite email — minted-but-dormant since w4/m12 — now redeems via `AcceptInvite` (REST `POST /v1/invites/accept`, GraphQL `acceptWorkspaceInvite`, MCP `accept_workspace_invite`): the dashboard's auth pages stash the token across the Kratos sign-up/login round-trip and the authenticated layout redeems it, so an invite works even when the recipient signs up under a **different email** than the one invited — the case the email match above can never redeem. The token path runs the same accept-time plan/seat guards (`AcceptInviteByToken` shares the redemption core with `AcceptInvitesForEmail`) but refuses **loudly** (named already-accepted / expired / plan-cannot-seat errors) where the login path skips silently — the caller asked explicitly, so they're told why. A redeemed token is single-use.

## Guardrails

- **Seat cap** — accepted members + outstanding invites both consume a seat, so a single-member (Hobby) workspace can't invite a second person until upgraded (`store.CanAddMember`).
- **Plan-gated roles** — invite/change-role reject a role the workspace's plan doesn't offer (`store.RoleAllowedOnPlan`, `guardPlanRole`); see "Roles are plan-gated" above.
- **Last-admin refusal** — the only admin cannot be demoted or removed, so a workspace is never left with no one who can administer it (`CountTenantAdmins`, `guardLastAdmin`).
- **Atomic membership** — the `tenant_members` row is the source of truth; its OpenFGA tuple is kept in step (grant on accept/upgrade, revoke on remove/downgrade). Postgres and OpenFGA aren't one transaction, so tuple writes are best-effort and idempotent (check-before-grant, delete-tolerates-absent).

## Enforcement (the definition of done)

On a cluster with OpenFGA enforced: an admin invites an email → the recipient gets the mail (Mailpit locally), signs up, and lands in the workspace at the assigned role; the [ADR012-auth.md](ADR012-auth.md) role matrix is observably enforced per role (a viewer reads but 403s on suspend / member management; a developer mutates resources but not org settings; an admin manages members); removing a member revokes their access; the last admin cannot remove or demote themselves.

## Member identity: `userId` + `email` (w6/m10)

`members.MemberView` (`lego/backend/internal/members/service.go`) carries `userId` (the opaque `own-` id) and `email` alongside `subject`/`role`/`createdAt` — the same enrichment `workspaces.ListMembers` performs for the owners read API ([ADR018-render-parity.md](ADR018-render-parity.md)): `OwnerIDForSubject` mints/resolves the `own-` id, an injected `EmailLookup` (adapted from the Kratos-admin `IdentityReader`, `BEX_KRATOS_ADMIN_URL`) resolves the email. REST, GraphQL (`workspaceMembers`), and MCP (`list_workspace_members`) all surface the same two fields off the one `List` verb — no per-surface reimplementation. Degradation is honest, not an error: with the identity reader unwired, or on a per-subject lookup miss, `email` comes back `""` and the request still succeeds; `userId` is always populated (it's minted from the control-plane store, not the identity provider). The dashboard Team page shows `email` as a member's primary identity, falling back to `userId` when email is unavailable, with the raw `subject` demoted to a secondary line.

**Mutations stay subject-keyed.** `ChangeRole`/`Remove` (REST `PATCH`/`DELETE .../members/{subject}`, the GraphQL mutations' `subject:` arg, the MCP tools' `subject` param) are unchanged — a deliberate call, not an oversight: this is a bex-native contract, not a mirror of Render's `userId`-keyed REST surface (see the owners API's note in `docs/render-artifacts/owners-api.md`).

## Parity completion (w1/m33, 2026-07-16)

The gaps a live side-by-side against Render's Team Members section surfaced, closed in one pass:

- **Seat usage** — `SeatUsage` (viewer-visible, like `List`) returns Render's `owner.usage.users {used, limit}`: `used` = accepted members + outstanding invites (the exact `store.CanAddMember` formula, so the display and the refusal can never disagree), `limit` from `PlanLimits.MaxMembers` (0 = unlimited). REST `GET /v1/workspaces/{id}/seat-usage`, GraphQL `workspaceSeatUsage`, MCP `get_workspace_seat_usage`; the Team card title renders "X of Y seats" and disables the invite entry point when full (the w6/m15 blocked-invite CTA stays as the recovery path).
- **`mfaEnabled` on `MemberView`** — Render's per-member `otpEnabled`, spelled the way bex's own owners read API already spells it. The members `IdentityLookup` seam now carries `{Email, MFAEnabled}` from the same Kratos-admin credential inspection (`workspaces.KratosIdentities.Lookup`, incl. the w4/020 webauthn-stub rule); honest-false on a miss, exactly like `email`. The member row shows a 2FA badge.
- **Audit rows for every member mutation** — invite / resend / change-role / remove / revoke record through the authorize interception with `member:<subject>` / `invite:<inv-id>` targets (`AuthorizeOnTarget`); Invite and ChangeRole defer their allowed row until the mutation lands (`RecordMemberInvited` / `RecordMemberRoleChanged`) so it can carry the invite id + email (`TargetName`) and the typed `roleFrom`→`roleTo` pair (migration 0040); both acceptance paths (login email-match in `tenancy.go`, token verb) record `members.AcceptInvite` with the **accepting** identity as the caller. REST audit metadata exposes `roleFrom`/`roleTo`/`email`.
- **Placement** — the dashboard Team panel lives on `/workspace/settings` (between the details card and the danger zone), where Render puts it; it had been on account `/settings` since w4/m12 (w5/m36 had recorded that as an IA choice — deliberately revisited by user direction, 2026-07-16).

## Known divergence from Render

- **No `name` trait.** bex's Kratos identity schema defines only `email` (no `name`), so nothing above ever surfaces a Render-style `user.name` — there is no field to omit-on-unset here, the trait simply doesn't exist. → future work only if a real consumer needs it. (Avatar likewise — bex has no avatar upload anywhere.)
- **Flatter GraphQL.** Render nests members under `owner.team.members`; bex has no polymorphic `owner` type, so it exposes workspace-scoped `workspaceMembers` / `workspaceInvites` queries. Field names (role, email, expiresAt, userId) match.
- **Active-workspace resolution** stays single-tenant (w1/m9): a caller resolves to one workspace for resource queries. The member verbs sidestep this by taking an explicit workspace id; a full workspace switcher is future work (the `workspaces` query already returns every membership).
