# Account deletion verification

Use an isolated `dev-2` stack. Never run this against a shared production identity or workspace. The workflow permanently deletes the Kratos identity and has no cancel operation after intent is recorded.

## Fixture

Create two verified human identities, A and B, in separate browser profiles. For A, prepare:

1. A sole-member workspace with a running whoami service and record its public URL, App CR, OpenBao paths, and billing/subscription identifiers.
2. A shared workspace where A and B are both admins and B can manage it.
3. A third workspace where A is the only admin and B is a non-admin member.
4. One bex API key, one SSH public key, a connected OAuth/device agent, and a second active Kratos browser session.

Record only opaque resource ids in the evidence file. Do not record cookies, tokens, private keys, `.env`, or kubeconfig contents.

## Blocker proof

Open `/settings#danger-zone` as A at desktop and narrow-mobile widths. Capture pending and ready states side by side and confirm the fifth navigation item, section bounds, headings, action placement, and card height remain stable.

The preview must classify the sole-member workspace as delete, the shared-admin workspace as leave, and the last-admin workspace as blocked. Enter `delete my account`; the button remains disabled and the API returns `ACCOUNT_DELETION_BLOCKED` with workspace id/name but no identity id.

Promote B to admin in the blocking workspace (or remove B/delete it), reload the preview, and confirm no blocker remains.

## Deletion and failure injection

Before each fault, snapshot the `account_deletions` row. Inject one failure at a time, restore the dependency, and restart bex-api where noted:

- Postgres unavailable while recording intent: nothing destructive occurs.
- OpenFGA revoke failure: membership rows and Kratos identity remain; retry converges after OpenFGA recovers.
- Hydra admin failure: API keys/grants remain in a retryable state and no workspace or Kratos identity is deleted.
- one pre-cascade workspace purger failure: the tenant row survives; retry uses the existing workspace teardown.
- Kratos session-delete failure and identity-delete failure: every prior cleanup is already idempotent and the `identity` state retries without onboarding.
- process restart after each durable state transition: the worker reclaims the row after its lease and resumes.

During every pending state, replay A's old browser/API requests. They must fail with `ACCOUNT_DELETION_PENDING`, and no new `tenants.owner_identity_id` row may appear for A.

## Convergence assertions

After the final retry:

- the tombstone is `done`; processing it again changes nothing;
- the sole-member tenant row, App CR, public route, secrets, datastore/sandbox artifacts, and hosted billing subscription are gone;
- both shared workspaces remain and B can manage them; A has no Postgres member row or OpenFGA tuple and surviving `owner_identity_id` bindings are null;
- A's API key cannot introspect, the SSH key cannot authenticate, connected OAuth credentials cannot call the API, push endpoints are gone, and both browser sessions fail Kratos `whoami`;
- retained audit, SSH-session, invite, registry-credential, and webhook provenance uses the `deleted:own-*` marker, never A's email/name/subject;
- Kratos identity GET returns 404 and repeating its DELETE is accepted as converged;
- registering the same email creates a different Kratos subject, a new `own-*` id, and one clean personal workspace with none of A's credentials, memberships, or notification state.

Attach the dated redacted command/output transcript and the two responsive screenshots to the milestone evidence before closing `.pm/w2/m84/t007.md`.
