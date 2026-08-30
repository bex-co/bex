# w2 · m84 — Delete user account: safe identity and workspace offboarding

**Worker:** worker2 **Goal:** a signed-in human can permanently delete their bex account from `/settings` without orphaning workspace access, recreating their personal workspace on retry, or leaving usable credentials behind **Status:** done

## Tasks (in order)

| id   | title                                                            | est | depends_on         |
| ---- | ---------------------------------------------------------------- | --- | ------------------ |
| t001 | Freeze the account-deletion contract and data-retention map — **DONE** | 45m | —            |
| t002 | Add durable deletion intent and suppress onboarding remint — **DONE** | 60m | t001         |
| t003 | Preview and execute safe workspace offboarding — **DONE**       | 60m | t002               |
| t004 | Revoke credentials and purge or anonymize subject data — **DONE** | 60m | t002             |
| t005 | Orchestrate retryable deletion and remove the Kratos identity last — **DONE** | 60m | t003, t004 |
| t006 | Add the account Settings danger-zone flow — **DONE**            | 60m | t005               |
| t007 | Prove deletion and retry behavior in the isolated full stack — **DONE** | 45m | t006        |
| t008 | Render parity — account-deletion surface and deliberate gaps — **DONE** | 30m | t007       |
| t009 | Simplify the code this milestone changed — **DONE**             | 30m | t008               |
| t010 | Test coverage for destructive account lifecycle — **DONE**      | 45m | t008               |
| t011 | Closeout — **DONE**                                             | 15m | t009, t010         |

## Definition of done

- `/settings` ends with a localized **Danger zone** section whose deletion preview names workspaces that will be deleted, workspaces the caller will leave, and any blocking workspace; its route skeleton matches the fifth section and fifth navigation item at desktop and narrow-mobile widths.
- Only a direct Kratos-session caller can request its own deletion. API keys, machine OAuth clients, and delegated human OAuth tokens cannot delete the identity they act for. The dashboard calls a bex-api Core verb; it does not independently delete Kratos state from SSR.
- A workspace where the caller is the only member is deleted through the existing `workspaces.Service` teardown, including external purgers. A workspace with another admin survives and loses only the deleting member. A workspace with other members but no other admin blocks deletion with a stable typed error and actionable workspace details.
- A durable deletion intent is written before destructive cleanup and makes onboarding fail closed for that subject, so `EnsureTenant` cannot mint a replacement personal workspace during a retry. Cleanup is idempotent and resumes after injected Postgres, OpenFGA, Hydra, workspace-purger, and Kratos failures.
- All subject-owned API keys, OAuth grants/tokens, SSH keys, push/notification preferences, transient connect state, memberships, and other classified account records are revoked or removed. Workspace-owned resources that survive the member's departure keep working; audit/provenance and legally required billing history follow the retention/anonymization policy recorded by t001.
- The Kratos admin `DELETE /admin/identities/{id}` call is the final irreversible step. Its `204` and already-absent `404` both converge to done, every Kratos session stops authenticating, and no post-identity cleanup remains that would require the user to retry.
- A real isolated-stack walk covers: a sole-member workspace with a running resource, a shared workspace with another admin, a blocked last-admin workspace, at least one API key, SSH key, connected agent, and second browser session. After resolving the blocker and confirming deletion, the sole-member workspace and its external artifacts are gone, the shared workspace remains manageable by its other admin, every credential/session is unusable, repeat processing is harmless, and signing up again with the same email receives a new identity and clean personal workspace.

## Source + Goal linkage

- **Source:** user request, 2026-08-29: research how to add delete-user-account to `https://dashboard.bex.co/settings`, then hand it to `/pm` for `w2`. Repository evidence: `dashboard/src/features/auth/pages/settings-page/index.tsx` currently has Account, Integrations, Access, and Security sections but no deletion surface; `workspaces.Service.Delete` already owns complete workspace teardown; `members.Service.Remove` owns FGA + membership convergence and last-admin refusal; `api.tenantService.EnsureTenant` auto-mints the personal workspace on every human cache miss; `.pm/FUTURE-MAYBE.md` explicitly named account deletion as a trigger to resolve creator-bound workspace offboarding. External evidence: Ory's current admin contract makes identity deletion permanent (`DELETE /admin/identities/{id}`, `204` or `404`) and separately supports deleting all identity sessions; Render exposes account deletion from the bottom of the user profile/settings page but no public account-delete operation in its documented API.
- **Goal linkage:** ADR008's open-source Render alternative and API-first/deterministic pillars. Account deletion is a baseline hosted-product lifecycle operation; putting orchestration in bex-api keeps the human UI and API on one retryable Core behavior instead of making the dashboard a second control plane.
- **Expected outcome:** a user can leave bex without support intervention, shared workspaces are not orphaned or silently destroyed, personal hosting resources and credentials are removed, and transient infrastructure failures cannot strand a half-deleted identity with live access.
- **Why now:** this is the explicit account-deletion request that `.pm/FUTURE-MAYBE.md` named as the trigger for solving creator-bound offboarding. The teardown pieces now exist (durable workspace deletion, membership/FGA revocation, API/SSH key revocation, session and connected-agent controls); coordinating them now is smaller and safer than adding an isolated Kratos delete button that bypasses those pieces.
- **Render parity:** **included** — this changes a user-facing dashboard/API surface. Match Render's discoverability and irreversible confirmation while recording deliberate bex differences: bex exposes a documented self-delete API because ADR008 forbids dashboard-only actions, and it must disclose multi-workspace disposition that Render's public docs/API do not specify.
