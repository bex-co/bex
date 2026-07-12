# Workspace members & roles (w4/m12)

The one multi-tenant IAM surface [w1/m9](ADR003-control-plane.md) left out: invite a teammate by email, assign one of Render's five roles, list members, change a role, and remove a member — over Core verbs that write `tenant_members` rows and OpenFGA `workspace:<id>` tuples together, with a dashboard **Settings → Team** page. It turns the role matrix OpenFGA has modelled since m4 ([ADR012-auth.md](ADR012-auth.md), `deploy/gitops/authz/model.fga`) from a static model into a populated one: a workspace stops being a single-person silo.

## The shape

One feature package, three thin adapters, Render-consistent — the same rule the rest of bex-api follows ([ADR006-bex-api.md](ADR006-bex-api.md)):

- **`lego/backend/internal/members`** — the Core verbs (`service.go`): `List`, `Invite`, `ChangeRole`, `Remove`, `ListInvites`, `RevokeInvite`. Every verb targets an **explicit workspace id** and gates on _that_ workspace (`AuthorizeOn(rel, workspace:<id>)`), like the workspace-lifecycle verbs — managing members is an admin acting **on** a workspace it administers, not on its own default workspace.
- **`rest.go` / `graphql.go` / `mcp.go`** — the fragments. REST under `/v1/workspaces/{id}/members` + `/invites` (a bex extension — Render manages members only via its dashboard GraphQL); GraphQL `workspaceMembers` / `workspaceInvites` + the four mutations (the dashboard Team page's client); MCP six tools (an agent seats teammates the same way a human does).
- **`lego/backend/internal/store/members.go`** — the write side of membership: role update / remove on `tenant_members`, the `tenant_invites` lifecycle, and `AcceptInvitesForEmail` (redemption). Reads (`ListTenantMembers`/`CountTenantMembers`) live in `store/workspaces.go`.
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

## Invites: pending row → redeem on login

An invite is addressed to an **email**, which has no OpenFGA subject yet — the recipient may not have signed up. So an invite is a `tenant_invites` row (id, email, role, token, `expires_at`, `accepted_at`), not a membership. It is redeemed on the recipient's **first authenticated login**:

1. The auth gate reads the caller's verified email from their Kratos identity traits (`internal/api/auth.go` `whoami`) and passes it to onboarding.
2. `tenantService.acceptInvites` (`internal/api/tenancy.go`) calls `AcceptInvitesForEmail`, which — in one transaction — turns every outstanding, unexpired invite for that email into a `tenant_members` row at its role and marks the invite accepted, then writes the matching OpenFGA role tuple.
3. Onboarding's **personal-tenant** resolution is owner-keyed (`CreateTenantWithMember`'s `owner_identity_id` upsert), **not** membership-keyed, so an invited membership never masquerades as the caller's own workspace — the admin grant stays pinned to the workspace the caller actually owns, never the one they were invited to as a viewer.

Invites expire (Render's 7 days). Delivery is best-effort: a flaky relay is logged, not surfaced — the invite row exists and redeems by email match regardless, so an admin can resend. Set `BEX_DASHBOARD_URL` to give the email a sign-up deep link.

## Guardrails

- **Seat cap** — accepted members + outstanding invites both consume a seat, so a single-member (Hobby) workspace can't invite a second person until upgraded (`store.CanAddMember`).
- **Last-admin refusal** — the only admin cannot be demoted or removed, so a workspace is never left with no one who can administer it (`CountTenantAdmins`, `guardLastAdmin`).
- **Atomic membership** — the `tenant_members` row is the source of truth; its OpenFGA tuple is kept in step (grant on accept/upgrade, revoke on remove/downgrade). Postgres and OpenFGA aren't one transaction, so tuple writes are best-effort and idempotent (check-before-grant, delete-tolerates-absent).

## Enforcement (the definition of done)

On a cluster with OpenFGA enforced: an admin invites an email → the recipient gets the mail (Mailpit locally), signs up, and lands in the workspace at the assigned role; the [ADR012-auth.md](ADR012-auth.md) role matrix is observably enforced per role (a viewer reads but 403s on suspend / member management; a developer mutates resources but not org settings; an admin manages members); removing a member revokes their access; the last admin cannot remove or demote themselves.

## Known divergence from Render

- **Members are keyed by identity subject, not `user{email,name}`.** Render's `owner.team.members[].user` carries email/avatar/OTP status; bex has no per-member profile store yet, so the members list surfaces the subject. Invites do carry the email (it's the redemption key). → future work.
- **Flatter GraphQL.** Render nests members under `owner.team.members`; bex has no polymorphic `owner` type, so it exposes workspace-scoped `workspaceMembers` / `workspaceInvites` queries. Field names (role, email, expiresAt) match.
- **Active-workspace resolution** stays single-tenant (w1/m9): a caller resolves to one workspace for resource queries. The member verbs sidestep this by taking an explicit workspace id; a full workspace switcher is future work (the `workspaces` query already returns every membership).
