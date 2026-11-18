# w2 · m479 — Services reach GCP through Workload Identity Federation without key files, as on Render

**Worker:** worker2 **Goal:** A Pro-or-higher service that sets `GCP_WORKLOAD_IDENTITY_PROVIDER` (and optionally `GCP_SERVICE_ACCOUNT_EMAIL`) gets a generated `external_account` credential configuration named by `GOOGLE_APPLICATION_CREDENTIALS`, so Google's libraries authenticate with the bex token and no service-account key. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Parse the provider, derive the audience, generate the credential configuration, mount it | 60m | w2/m477/t004 |
| t002 | Golden tests against `gcloud` and `google-auth` | 45m | t001 |
| t003 | Production acceptance (user gate: a GCP project with a pool and provider) | 30m | t002 |
| t004 | Render parity | 20m | t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 30m | t004 |
| t007 | Closeout | 10m | t005, t006 |

## Definition of done

- A Pro-workspace service with `GCP_WORKLOAD_IDENTITY_PROVIDER` (resource name or full URL) and optional `GCP_SERVICE_ACCOUNT_EMAIL` gets `GOOGLE_APPLICATION_CREDENTIALS` naming a generated `external_account` configuration that matches `gcloud iam workload-identity-pools create-cred-config` output for the same inputs (golden test), with `credential_source.file` pointing at the refreshed token.
- A project id where the project number is required fails with a named deploy-log line instead of minting for the wrong audience.
- On production, once the user's GCP gate is met, the service gets a `cloud-platform`-scoped access token through federation.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w2` (2026-11-17); the user filed all five proposed milestones with `/pm them all to w2` the same day. That reopens the 2026-08-23 deferral recorded in [`.pm/FUTURE-MAYBE.md`](../../FUTURE-MAYBE.md) (from `w2/025`) under the Render-parity-first rule (AGENTS.md, user decisions 2026-10-17/2026-10-20; precedent: `w2/m252` absorbed a FUTURE-MAYBE item under the same rule). Ledger: [ADR018](../../../docs/ADR018-render-parity.md) row "Managed OIDC for AI providers" (✖ on REST/GraphQL/MCP/UI, the last all-✖ row that is not an anti-goal) and [ADR030](../../../docs/ADR030-pricing.md)'s pricing-parity table ("Open gap | OIDC authentication"). Render's page lists GCP beside AWS, Anthropic and OpenAI; ADR018's row omitted it until w2/m476 t001.
- **Goal linkage:** Render parity first (ADR008).
- **Expected outcome:** All four providers Render documents work on bex.
- **Why now:** With w2/m477's mechanism in place, GCP's only distinct piece is the generated credential configuration; finishing it closes the provider list.
- **Render parity:** Included: env-var behavior on every surface and the runtime variable compared with Render's page.
- **Depends on:** w2/m477 (delivery mechanism).
- **User gate:** t003 needs a GCP project; see the task.
