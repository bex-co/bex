# ADR086 — Account deletion: durable identity and workspace offboarding

**Status:** Accepted (2026-08-29). Source: `.pm/w2/m84`. Composes with [ADR003](ADR003-control-plane.md), [ADR012](ADR012-auth.md), [ADR024](ADR024-members.md), and [ADR075](ADR075-user-onboarding.md).

## Context

Deleting only an Ory Kratos identity would leave live memberships, Hydra credentials, SSH keys, notification endpoints, and creator references. Deleting a personal workspace first without recording the account lifecycle is also unsafe: normal human-request middleware calls `EnsureTenant`, which would mint a replacement workspace for the still-valid identity on the next request.

Account deletion is therefore a control-plane workflow, not a dashboard call to Kratos. It must preserve shared workspaces, reuse established authorization and teardown mechanisms, fail closed once it begins, and tolerate a restart between any two steps.

## Decision

### Public contract and authorization

The Core feature is `accounts` and exposes:

- `GET /v1/users/deletion-preview` and GraphQL `accountDeletionPreview`, classifying every workspace as `delete`, `leave`, or `blocked`.
- `DELETE /v1/users` with `{ "confirmation": "delete my account" }` and GraphQL `deleteAccount(confirmation: String!)`, durably requesting deletion and returning its state. The internal begin operation is idempotent; after intent commits, public requests from an old session fail closed with `ACCOUNT_DELETION_PENDING`.
- no MCP tool. An irreversible identity-boundary operation is intentionally unavailable to autonomous tool callers.

Every verb begins with the normal `can_view` Core authorization check. It then requires `Identity.Method == "session"` and `Identity.Human == true`; an API key, machine OAuth client, or delegated human OAuth token is refused even when it represents the same subject. Confirmation is case-sensitive and exact.

| code | status | meaning |
| --- | --- | --- |
| `ACCOUNT_DELETION_BLOCKED` | 409 | one or more workspaces would be orphaned |
| `ACCOUNT_DELETION_PENDING` | 409 | onboarding or an ordinary request followed a recorded intent |
| `ACCOUNT_DELETION_UNAVAILABLE` | 503 | a required deletion dependency is unavailable |

`ACCOUNT_DELETION_BLOCKED` includes workspace ids and display names, never Kratos identity ids. Its guidance is to promote another member, remove the other members, or delete the workspace first.

### Workspace disposition

Preview and the intent transaction apply the same matrix. The transaction locks the subject's memberships in deterministic workspace-id order and rechecks it; a stale preview never authorizes destruction.

| membership state | disposition | result |
| --- | --- | --- |
| caller is the only member | `delete` | invoke existing `workspaces.Service` teardown, including external purgers |
| another admin remains | `leave` | revoke OpenFGA membership, remove the member row and member-cascaded data; clear `tenants.owner_identity_id` if it names the caller |
| other members remain, but none is another admin | `blocked` | write nothing destructive; return `ACCOUNT_DELETION_BLOCKED` with actionable workspace details |

This is deletion-specific offboarding, not general workspace ownership transfer. A surviving workspace is administered by its remaining admin; an owner-identity binding is an onboarding uniqueness key and is cleared rather than reassigned.

### Durable state machine and ordering

`account_deletions` is a subject-keyed tombstone in the control-plane database. It contains state, the immutable anonymization marker and workspace plan, retry/claim timestamps, attempt count, and a bounded sanitized error. It never contains an email, display name, session, OAuth token, or bearer credential. The begin transaction deletes pending invites addressed to the normalized account email and anonymizes accepted invite/audit history after inserting the intent but before the atomic commit, so this PII never has to survive in worker state.

| state | durable work | next state |
| --- | --- | --- |
| absent | authorize, preflight and atomically insert intent | `pending` |
| `pending` | claim with a lease; revoke credentials and transient state | `cleaning` |
| `cleaning` | delete sole-member workspaces; leave eligible shared workspaces; anonymize retained provenance | `identity` |
| `identity` | revoke all Kratos sessions, then delete the identity | `done` |
| `done` | no work; tombstone remains | `done` |

Every operation is idempotent and a worker resumes from stored state after Postgres, OpenFGA, Hydra, a workspace purger, or Kratos fails. Failure releases the claim, stores only a bounded operator-safe reason, and schedules retry with backoff. Operators may inspect and retry the row; users cannot cancel once intent is recorded.

Two ordering invariants are absolute:

1. Intent commits before cleanup. `EnsureTenant` consults the tombstone before cache use, invite redemption, or personal-workspace creation and returns `ACCOUNT_DELETION_PENDING`. This prevents workspace resurrection while the identity can still present a session.
2. Kratos identity deletion is last. Before it, every failure is recoverable by the durable worker without asking the user to authenticate again. Kratos delete treats both `204` and `404` as success; afterward no cleanup remains. The `done` tombstone survives so the old subject can never be onboarded again.

### Data disposition

The opaque deleted marker is `deleted:<own-id>`. It retains event correlation but contains no email, name, or active Kratos subject.

| store / field | disposition | rationale |
| --- | --- | --- |
| `tenant_members.subject` | remove through member/workspace teardown | authorization source; dependent reconciliation and push rows cascade |
| `tenants.owner_identity_id` | null on surviving workspace; workspace cascade otherwise | prevent remint binding without ownership transfer |
| `owner_ids.subject` | replace with deleted marker and retain `own-*` mapping | keep public provenance stable without active subject |
| `notification_settings.subject` | hard-delete | personal preference; no FK cascade |
| `ssh_keys.subject` | hard-delete before identity deletion | bearer access credential |
| `ssh_sessions.subject` | anonymize | operational history, not active credential |
| `github_connect_transactions.subject` | hard-delete | transient authorization transaction |
| `oauth_revocations.subject` | replace with deleted marker and retain | retain fail-closed revocation history without an active subject |
| `device_push_subscriptions`, `webpush_subscriptions`, `push_notifications`, `push_deliveries`, `membership_role_reconciliations` | membership FK cascade | member-scoped state |
| `tenant_invites.invited_by` | anonymize | retain inviter provenance |
| `tenant_invites.email` | delete pending invites addressed to the account; anonymize accepted history | same-email registration must not inherit an old invitation or retained PII |
| `registry_credentials.created_by`, `webhook_endpoints.created_by` | anonymize for surviving workspace; cascade otherwise | workspace resources continue without exposing subject |
| `audit_events.caller` and email-valued `target_name` | anonymize | retain security history with identity and address severed |
| Hydra API-key clients marked `bex.co/created-by` | unbind from workspace/OpenFGA, then delete | remove subject-owned machine credentials safely |
| Hydra grants, access/refresh tokens, consent/login sessions | revoke/delete through Hydra admin | delegated access must stop independently of Kratos |
| Kratos sessions | delete immediately before identity | invalidate every browser session |
| Kratos identity | delete last; `204` and `404` converge | final irreversible record |
| Stripe customer/subscription/invoice/usage history | retain under legal policy; sever active bex access links | financial records are not identity credentials |

The dropped `mcp_workspace_selections` table has no live disposition. Future subject/provenance columns must extend this table and cleanup before shipping.

### Dashboard behavior

Account Settings ends with a localized fifth section and navigation item named **Danger zone**. It names workspaces to delete or leave, lists blockers and recovery choices, and requires the exact confirmation phrase. Its pending skeleton preserves the fifth navigation item, section bounds, heading/action placement, and responsive layout.

After acceptance, the dashboard signs out locally and shows a non-retrying completion state. It never calls Kratos admin. Old-session requests fail closed with `ACCOUNT_DELETION_PENDING`; the worker owns convergence.

## Recovery and verification

Operators inspect `account_deletions`, repair the dependency, and let the worker retry or restart the API. They must not remove a pending tombstone to restore login because that can resurrect a personal workspace. A `done` row is permanent absent separately reviewed subject-id collision recovery.

Release proof uses an isolated real stack with a sole-member workspace and running resource, a shared workspace with another admin, a blocked last-admin workspace, API and SSH keys, a connected OAuth agent, and a second browser. It injects failures at each external seam and proves restart resumption, repeat processing, credential/session invalidation, external cleanup, shared-workspace survival, and same-email registration producing a new subject and clean personal workspace.

## Consequences

- Account deletion is asynchronous after one durable authorized request.
- The tombstone is retained and suppresses onboarding for the old subject.
- Shared workspace resources survive and retained creator labels are anonymized.
- A last-admin blocker must be resolved before deletion begins.
- General ownership transfer and personal-data export remain out of scope.
