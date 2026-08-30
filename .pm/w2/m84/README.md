# w2 · m84 — Self-service account deletion from `/settings`

**Worker:** worker2 **Goal:** let a signed-in human permanently delete their bex account without leaving live credentials, memberships, authorization tuples, or ownerless resources behind **Status:** todo

## Tasks (in order)

| id   | title                                                               | est  | depends_on             |
| ---- | ------------------------------------------------------------------- | ---- | ---------------------- |
| t001 | Settle the account-deletion contract and complete the subject inventory | 1h   | —                      |
| t002 | Build an idempotent account-offboarding coordinator                 | 2h   | t001                   |
| t003 | Revoke Hydra, SSH, push, and Kratos access in a safe terminal order | 1h30 | t002                   |
| t004 | Expose a human-only GraphQL delete-account mutation                 | 45m  | t002, t003             |
| t005 | Add the `/settings` account danger zone and deletion flow           | 1h30 | t004                   |
| t006 | Render parity and retention-policy record                           | 30m  | t001, t005             |
| t007 | Simplify the code this milestone changed                            | 30m  | t006                   |
| t008 | Test coverage for complete and interrupted deletion                | 1h30 | t002, t003, t004, t005 |
| t009 | Closeout                                                            | 30m  | t007, t008             |

## Definition of done

- A directly authenticated human can start account deletion from a clearly separated danger zone on `https://dashboard.bex.co/settings`; machine API keys and delegated OAuth clients cannot invoke it.
- The server reports actionable blockers before destructive work. In particular, it never silently destroys a workspace shared with other people and never removes the last administrator or the `owner_identity_id` anchor from a surviving workspace.
- Once accepted, deletion is retry-safe and reaches one terminal outcome: all subject-owned durable credentials and active access are revoked, the subject's removable control-plane data and membership/FGA bindings are gone, sole-user personal resources follow the existing workspace-delete purge path, and the Kratos identity is deleted last.
- Retained audit/history rows preserve non-secret opaque attribution rather than being rewritten or erased; the retention choice is documented.
- The browser loses its session/cache and lands on the signed-out state. A later login with the deleted identity cannot recover access.
- Pending and ready `/settings` geometry still match at desktop and narrow-mobile widths.

## Research handoff

- `dashboard/src/features/auth/pages/settings-page/index.tsx` currently delegates profile/password to Ory and has no danger zone. `dashboard/src/routes/settings.tsx` already requires auth; `logout-page` and `invalidateSessionCache()` are the session-cleanup precedent.
- `lego/backend/internal/workspaces/kratos.go` is read-only today. Ory's admin API provides permanent identity deletion (`DELETE /admin/identities/{id}`) and session deletion (`DELETE /admin/identities/{id}/sessions`), but calling those alone is incorrect for bex.
- A subject may appear in `tenant_members`, `tenants.owner_identity_id`, `owner_ids`, notification/push rows, `ssh_keys`, active/historical `ssh_sessions`, `github_connect_transactions`, and `oauth_revocations`. Hydra API-key clients carry the minting subject in `bex.co/created-by` and have their own `tenant_members` + OpenFGA bindings.
- `workspaces.Service.Delete` already owns the correct resource purge order for a sole-user personal workspace. `members.Service.Remove` protects the last admin but is admin-driven and is not a complete self-offboarding operation.
- The operation crosses Postgres, OpenFGA, Hydra, Kratos, and workspace purgers. Do not delete the Kratos identity first: that removes the caller's ability to retry while leaving the harder control-plane cleanup stranded.

## Source + Goal linkage

- **Source:** user request, 2026-08-29: research adding “delete user account” to `dashboard.bex.co/settings` and hand it to w2. Promotes the account-deletion trigger recorded in `.pm/FUTURE-MAYBE.md`.
- **Goal linkage:** ADR008's trustworthy Render-alternative experience, ADR012's Kratos/Hydra ownership boundary, ADR024's member lifecycle, and ADR028+ security lineage for durable-credential revocation.
- **Expected outcome:** a user can leave bex completely from Settings, while collaborators, resources, credentials, and retained audit evidence end in an explicit and safe state.
- **Why now:** production QA already leaves disposable Kratos identities and empty orphan workspaces because no self-service deletion path exists; a naive Kratos-only button would make that residue worse.
- **Render parity:** **included.** Render's public docs expose leaving a workspace and retain deleted-user IDs in audit logs, but no public account-delete API was found. Record a fresh live Dashboard comparison and keep this as a deliberate dashboard/GraphQL-only bex extension if that remains true.
