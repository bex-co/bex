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
| `contributor` | + logs, restart/suspend/resume, bare redeploy, cron **schedule**; **not** create/delete, sensitive fields, or **choosing the code a workload runs** (w1/m68) |
| `developer` | + create/delete, connection strings, env vars, API keys; **no** org settings |
| `admin` | full resources **and** org settings — members, billing, protected envs |
| `billing` | billing management only (Stripe onboarding/checkout/portal via `can_manage_billing`) + read-only non-sensitive names |

Roles are **UPPERCASE on the wire** (Render's enum: `ADMIN`, `DEVELOPER`, …) and lowercase as the stored role / FGA relation; adapters convert at the view boundary. A change of role is a Revoke of the old relation tuple then a Grant of the new one; the `tenant_members.role` column is the source of truth, the tuple a best-effort follow-up (the resolver re-drives a missing grant on login).

### The contributor boundary: lifecycle, not code (w1/m68)

`contributor` is the "operate what exists" role, so the line bex draws is **whether the request supplies executable content**. Supplying a pre-deploy command, a cron `command`, a one-off job command, or a deploy `imageUrl`/`commitId` is create-like and needs `can_create` (developer and up); a parameter-free redeploy and a schedule-only cron change stay `can_operate`. The reasoning, and the verbs it covers, live in `lego/backend/internal/api/m68_executable_selection_test.go`, which enumerates the class so a new verb of the same shape cannot join it unnoticed — the failure mode that left four verbs behind when the class was first fixed.

**Parity note, honestly stated.** Render's dashboard descriptions for CONTRIBUTOR are not capturable from this workspace's tier (`docs/render-artifacts/team-members.graphql`: Contributor/Viewer/Billing are Scale+-gated and not selectable), so bex's exact boundary for these parameters is **not verified against Render's**. Render's public docs describe Contributor as able to deploy, which bex preserves — a bare trigger still works. What is unverified is whether Render also lets a Contributor swap the image or pin an arbitrary commit. bex deliberately does not, because that is attacker-chosen code running with the service's secrets and network identity. If a Scale-tier capture ever shows Render permitting it, this is a **deliberate divergence to keep**, not a bug to close.

The `billing` role's distinguishing capability became **live and enforced in w1/m60**: the customer-billing verbs (`internal/billing`'s `Status`/`Checkout`/`Portal`) now gate on `can_manage_billing` (`billing or admin`), so a billing-role member manages billing without workspace-admin — exactly Render's BILLING role — while staying denied every resource mutation and sensitive read. Before w1/m60 the relation was modelled in `model.fga` but had **no Go consumer** (billing verbs gated on admin-only `can_manage`), so the role's one distinguishing power was inert; the fix is the last modelled-but-dead relation gaining its consumer ([ADR012-auth.md](ADR012-auth.md), [ADR040-billing-metronome.md](ADR040-billing-metronome.md)).

### Roles are plan-gated (w6/m12)

Not every role is assignable on every plan — Render's live capture (`docs/render-artifacts/team-members.graphql`, `RESEARCH-workspaces.md` finding 5) shows the role picker itself narrows by plan: **Hobby** has no invites at all (single member, always `admin`); **Pro** offers `admin`/`developer` only; **Scale**/**Enterprise** add `contributor`/`viewer`/`billing`. bex encodes this as `PlanLimits.AllowedRoles` (`lego/backend/internal/store/plans.go`'s `LimitsFor`, alongside the member/service caps) plus a `RoleAllowedOnPlan(plan, role) bool` predicate, and enforces it in two places:

- `members.Invite` / `members.ChangeRole` reject a role outside `RoleAllowedOnPlan(tenant.Plan, role)` with a `core.ErrBadRequest` naming the plan and its allowed roles (`guardPlanRole`).
- `workspaces.Service.ChangePlan`'s downgrade guard refuses a plan change while any member holds a role the **target** plan's `AllowedRoles` no longer offers, naming the blocking member(s) — a downgrade must shed the disallowed roles first (`ChangeRole`/`Remove` them) before the plan can flip.

No migration is needed for pre-existing data: roles shipped after plans (w4/m12 came after w6/m1), so no workspace could have assigned an out-of-plan role before this guard existed.

## Invites: pending row → redeem on login

An invite is addressed to an **email**, which has no OpenFGA subject yet — the recipient may not have signed up. So an invite is a `tenant_invites` row (id, email, role, `token_hash`, `expires_at`, `accepted_at`), not a membership. The link token is stored **hashed** (sha256, w1/041 — a DB read yields no redeemable links); the plaintext exists only in flight, minted at create/resend and carried just long enough to email. It is redeemed on the recipient's **first authenticated login**:

1. The auth gate reads the caller's email from their Kratos identity traits (`internal/api/auth.go` `whoami`) and passes it — with its Kratos verified-state (`EmailVerified`) — to onboarding.
2. `tenantService.acceptInvites` (`internal/api/tenancy.go`) calls `AcceptInvitesForEmail`, which — in one transaction — turns every outstanding, unexpired invite for that email into a `tenant_members` row at its role and marks the invite accepted, then writes the matching OpenFGA role tuple.
3. Onboarding's **personal-tenant** resolution is owner-keyed (`CreateTenantWithMember`'s `owner_identity_id` upsert), **not** membership-keyed, so an invited membership never masquerades as the caller's own workspace — the admin grant stays pinned to the workspace the caller actually owns, never the one they were invited to as a viewer.

### An invite never re-roles an existing member (w1/m82)

**Inviting someone who already belongs to the workspace is refused** — `members.Invite` returns a conflict (HTTP 409 on REST, the same coded error on GraphQL/MCP) carrying `MEMBER_ALREADY_EXISTS`, matching Render's "already a member". Changing a member's role has exactly one verb, `ChangeRole`, which is audited (`roleFrom`→`roleTo`) and refuses to strip the last admin. Membership is keyed by subject while the address lives in Kratos, so the guard resolves the workspace's members through the same `Identities` seam `List` uses; without an identity reader (`BEX_KRATOS_ADMIN_URL` unset) no address resolves and the invite proceeds, because the redemption rule below holds the line regardless.

**Redemption creates memberships; it never changes one.** `store.redeemInvite` upserts `ON CONFLICT … DO UPDATE SET role = tenant_members.role` — a deliberate no-op update that keeps the existing role while still returning it — and both acceptance paths report that **effective** role, which is what the caller writes into OpenFGA, so the row and the tuple cannot disagree. The invite is still marked accepted so it stops lingering.

This replaces the original "an invite can UPGRADE an existing membership's role" behavior, which was the same write in the safe direction: `DO UPDATE SET role = EXCLUDED.role` also **silently downgraded**. Found in the 2026-08-19 dashboard QA walk — inviting an established **admin** at the invite dialog's default role demoted them on their next authenticated request (the auth gate redeems by email match, so the change landed later and nowhere the UI showed), reported to the inviter only as "Invitation sent". A role change is now impossible to trigger by accident from the invite path.

Invites expire (Render's 7 days). Delivery is best-effort: a flaky relay is logged, not surfaced — the invite row exists and redeems by email match regardless, and an admin can **resend** (`ResendInvite`, w1/m33: fresh mail, expiry pushed forward, id unchanged; a lapsed-but-unaccepted invite is revived). Since w1/041 a resend **rotates the token**: only the hash is at rest, so the original plaintext cannot be re-emailed — the freshly emailed link supersedes the original, which stops redeeming (before w1/041, the token was stored plaintext and resend kept it, so the original link stayed live). Set `BEX_DASHBOARD_URL` to give the email a sign-up deep link.

### Email verification and login-time redemption (w1/m53)

Login-time redemption (path 1–2 above) matches on the caller's Kratos **trait email**. Kratos issues a session **before** an address is verified (verification is an async flow), so on its own the trait email is not proof of ownership: if an invite is sent to an address whose owner has **not signed up yet**, an attacker could register with that address, get a session, and redeem the invite on first login — email is a unique Kratos identifier, so this window exists only for not-yet-registered invitees, but the invited role can be admin.

`whoami` therefore also reads Kratos's `verifiable_addresses` and reports `Identity.EmailVerified`. Setting **`BEX_REQUIRE_VERIFIED_INVITE_EMAIL=1`** gates login-time redemption on a verified email (`tenantService.RequireVerifiedInviteEmail`), closing the window. The gate shipped **off by default** (w1/m53) while the email-verification UX was incomplete; with the dashboard's `/auth/verification` page (w4/008, Ory Elements `Verification` over Kratos's `use: code` flow) completing the loop, **prod now sets it** (w1/040, `lego/operator/config/api/deployment.yaml`). A gate-skipped invite is not lost — it stays pending and redeems on the invitee's next login after they verify (the redemption runs on every resolver cache-miss). Deployments without a working verification UX must leave the flag unset or every invited teammate is blocked. The token path (below) is unaffected — its capability is the unguessable token, not an email claim.

**The emailed link is directly redeemable (w1/m33, dedicated handoff w11/m8).** Email now points to the exact HTTPS handoff `https://dashboard.bex.co/invite#invite=<token>`, keeping the bearer out of the initial HTTP request; an OS-verified native association or the dashboard fallback may claim it. The dashboard accepts exactly one 32-character lowercase-hex token, stores only that bearer capability in tab-scoped `sessionStorage`, applies a no-referrer policy, and scrubs the complete query/fragment before routing into sign-up or the authenticated dashboard. The token survives the Kratos/OAuth round-trip and redeems via `AcceptInvite` (REST `POST /v1/invites/accept`, GraphQL `acceptWorkspaceInvite`, MCP `accept_workspace_invite`), so an invite works even when the recipient signs up under a **different email** than the one invited — the case the email match above can never redeem. The token path runs the same accept-time plan/seat guards (`AcceptInviteByToken` shares the redemption core with `AcceptInvitesForEmail`) but refuses **loudly** where the login path skips silently. GraphQL exposes stable terminal codes `INVITE_INVALID`, `INVITE_EXPIRED`, `INVITE_ALREADY_ACCEPTED`, and `INVITE_PLAN_LIMIT`; typed store causes, not prose matching, drive them. A redeemed token is single-use. None of this changes verified-email gating for login-time email matching.

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
