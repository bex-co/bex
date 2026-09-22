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

`account_deletions` is a subject-keyed tombstone in the control-plane database. It contains state, the immutable anonymization marker and workspace plan, retry/claim timestamps, attempt count, and a bounded sanitized error. It never contains an email, display name, session, OAuth token, or bearer credential. The begin transaction deletes pending invites addressed to the normalized account email and anonymizes matching accepted invite, audit, workspace billing, and creation-attempt addresses after inserting the intent but before the atomic commit, so this PII never has to survive in worker state. It also retires nonterminal workspace-creation attempts. Subject cleanup later replaces every owned attempt's subject and billing address, including an address different from the account email; provider correlations remain available for financial cleanup.

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
| `account_deletions.subject` | retain permanently | fail-closed tombstone prevents re-onboarding the deleted subject; contains no email or credential |
| `tenant_members.subject` | remove through member/workspace teardown | authorization source; dependent reconciliation and push rows cascade |
| API keys (`CreatedBy`) | hard-delete: unbind, then delete the Hydra client | a key is a delegation of the subject's authority, so it cannot outlive the subject. **No longer deletion-only (w2/m163):** the same `apikeys.AccountTeardown` path now runs on every membership exit, narrowed to the workspace being left ([ADR024](ADR024-members.md) § Guardrails). Account deletion stays **global by subject** — narrowing it here would leave a deleted account's keys alive in every workspace but one |
| `tenants.owner_identity_id` | null on surviving workspace; workspace cascade otherwise | prevent remint binding without ownership transfer |
| `tenants.billing_email` | anonymize matching account email on surviving workspaces; workspace deletion removes the row | alternate workspace-owned billing contacts remain financial contact data, independent of the departing member |
| `workspace_creation_attempts.owner_subject`, `.billing_email` | anonymize subject and every owned attempt address; also anonymize matching account email at intent time | keep provider customer/setup/payment/subscription correlations and cleanup state for retries; terminal rows still expire after 30 days, rather than deleting a financial-cleanup capability early |
| `owner_ids.subject` | replace with deleted marker and retain `own-*` mapping | keep public provenance stable without active subject |
| `notification_settings.subject` | hard-delete | personal preference; no FK cascade |
| `ssh_keys.subject` | hard-delete before identity deletion | bearer access credential |
| `ssh_sessions.subject` | anonymize | operational history, not active credential |
| `github_connect_transactions.subject` | hard-delete | transient authorization transaction |
| `github_claim_selections.subject` | hard-delete | the same class ([ADR078](ADR078-github-workspace-connections.md) §3a, w2/m162): a short-lived, single-use, subject-bound memo of a completed proof. Anonymizing would leave a selection nobody may complete but the row still names a workspace; the table also CASCADEs on its workspace, because expiry alone is not retention — its sweep is piggybacked on writes |
| `cli_telemetry_events.subject` | hard-delete | usage telemetry, not security history; the row's stable `installation_id` would re-link an anonymized subject |
| `deploys.triggered_by` | anonymize on surviving workspace resources; resource/workspace cascade otherwise | trigger provenance must not retain an active identity after deletion |
| `webhook_delivery_attempts.requested_by` | anonymize manual replay provenance; endpoint/workspace cascade otherwise | immutable evidence permits only replacing this field with the subject's recorded deletion marker, with every other field unchanged; replay idempotency and delivery evidence survive |
| `product_activity_events.actor_id`, `.actor_type` | clear identifier and set actor type to unknown; workspace cascade on workspace deletion | retain bounded resource-adoption facts without a creator identity; recorder checks the deletion tombstone under the subject lock |
| `oauth_revocations.client_id` | retain | OAuth application correlation and permanent machine revocations remain meaningful after identity cleanup; a client identifier is not a bearer secret |
| `oauth_revocations.subject` | replace with deleted marker and retain | retain fail-closed revocation history without an active subject |
| `device_push_subscriptions`, `webpush_subscriptions`, `push_notifications`, `push_deliveries`, `membership_role_reconciliations` | membership FK cascade | member-scoped state |
| `tenant_invites.invited_by` | anonymize | retain inviter provenance |
| `tenant_invites.email` | delete pending invites addressed to the account; anonymize accepted history | same-email registration must not inherit an old invitation or retained PII |
| `registry_credentials.created_by`, `webhook_endpoints.created_by` | anonymize for surviving workspace; cascade otherwise | workspace resources continue without exposing subject |
| `audit_events.caller` and email-valued `target_name` | anonymize | retain security history with identity and address severed |
| `audit_events.caller_method`, `.oauth_client_id` | retain for the audit retention window | method is a category; OAuth application provenance does not restore the anonymized human caller |
| `billing_export_issues.actor` | retain on account deletion; workspace cascade on workspace deletion | internal control-plane callers supply an operator label for financial repair provenance; it is not an authenticated customer identity binding |
| Hydra API-key clients marked `bex.co/created-by` | unbind from workspace/OpenFGA, then delete | remove subject-owned machine credentials safely |
| Hydra grants, access/refresh tokens, consent/login sessions | revoke/delete through Hydra admin | delegated access must stop independently of Kratos |
| Kratos sessions | delete immediately before identity | invalidate every browser session |
| Kratos identity | delete last; `204` and `404` converge | final irreversible record |
| Stripe customer/subscription/invoice/usage history | retain under legal policy; sever active bex access links | financial records are not identity credentials |

The dropped `mcp_workspace_selections` table has no live disposition. Future subject/provenance columns must extend this table and cleanup before shipping.

### Schema coverage contract

This ADR is authoritative for **policy**. The real-PostgreSQL censuses enforce **coverage**; an inventory entry may never promise less cleanup than its ADR row. Update the policy, implementation, and inventory together. The tests inspect the migrated schema, so an FK added after a table's original migration counts, while removing that FK exposes the gap.

- `TestAccountDeletionInventoryPG` in `lego/backend/internal/store/accounts_pg_test.go` enumerates base-table columns by shape: `subject`, `email`, `identity_id`, `user_id`, and `client_id` with optional prefixes; every `*_by`; and `actor*`/`caller*`. Each has a `delete`, `anonymize`, `cascade`, or `retain` disposition with a reason. Broad matches such as `actor_type` and `caller_method` still require an explicit declaration.
- The tenant census in `lego/backend/internal/store/tenant_scope_census_pg_test.go` enumerates `tenant_id`, `workspace_id`, and `owner_id`. Each column must have a validated `ON DELETE CASCADE` path for that exact key to `tenants.id`, a declared registered workspace purger, or explicit retention. A cascade on another column is insufficient; transitive membership/notification cascades are valid. Composite `MATCH SIMPLE` foreign keys must have non-null sibling columns so nulls cannot bypass the constraint; `MATCH FULL` also preserves that invariant.
- Both guards share schema enumeration and inventory validation. Transactional mutation tests add undeclared tables/columns to the real schema and require readable rejection, then roll back. Stale declarations also fail, so removed columns cannot leave misleading policy entries.

Workspace teardown cascades pure workspace state, including one-off `jobs`, `datastore_event_facts`, `datastore_observed_checkpoints`, and `service_event_index`. The migration removes existing orphan rows before validating the new constraints. An audit arriving after workspace deletion remains audit history without recreating a workspace event projection. External resources still use the existing registered purgers before the tenant row is deleted; the SQL census does not replace that lifecycle.

The following SQL tables deliberately survive tenant deletion:

| table | retention and reason |
| --- | --- |
| `audit_events` | security history; startup and daily audit sweeps use `BEX_AUDIT_RETENTION_DAYS` (90 days by default); account deletion anonymizes identity/address fields |
| `ssh_sessions` | operational history under the same audit retention sweep; account deletion anonymizes subject |
| `cli_telemetry_events` | diagnostics under the same retention sweep; account deletion deletes the subject's rows, including linkable installation identifiers |
| `workspace_creation_attempts` | provisional workspace IDs precede tenant existence, so no tenant FK is possible; retain provider correlations for financial compensation and retry, anonymize account fields, and sweep terminal rows after 30 days (batches of 100). The 15-minute cleaner runs when workspace billing is configured; disabling it pauses reclamation |
| `agent_session_dispatches` | durable recovery intent must survive session deletion to catch late sandbox creation and egress-policy writes. Normal binding or completed predecessor cleanup removes the corresponding intent; generic abandoned tombstones have **no age-based GC** and retry indefinitely. After tenant deletion the workspace sandbox key is gone, so those retries cannot recover through that key; workspace sandbox/namespace purgers own teardown. This is explicit safety retention, not a claim that the worker bounds table size |

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
