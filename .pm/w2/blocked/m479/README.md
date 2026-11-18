# w2 · m479 — Services reach GCP through Workload Identity Federation without key files, as on Render

**Worker:** worker2 **Goal:** A Pro-or-higher service that sets `GCP_WORKLOAD_IDENTITY_PROVIDER` (and optionally `GCP_SERVICE_ACCOUNT_EMAIL`) gets a generated `external_account` credential configuration named by `GOOGLE_APPLICATION_CREDENTIALS`, so Google's libraries authenticate with the bex token and no service-account key. **Status:** blocked (t001, t002, t005, t006 done; t003 needs the user's GCP project with a workload identity pool and provider and a production pin carrying this work, then t004, t007)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Parse the provider, derive the audience, generate the credential configuration, mount it — **DONE** | 60m | w2/m477/t004 |
| t002 | Golden tests against `gcloud` and `google-auth` — **DONE** | 45m | t001 |
| t003 | Production acceptance (user gate: a GCP project with a pool and provider) | 30m | t002 |
| t004 | Render parity | 20m | t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 30m | t004 |
| t007 | Closeout | 10m | t005, t006 |

## Definition of done

- A Pro-workspace service with `GCP_WORKLOAD_IDENTITY_PROVIDER` (resource name or full URL) and optional `GCP_SERVICE_ACCOUNT_EMAIL` gets `GOOGLE_APPLICATION_CREDENTIALS` naming a generated `external_account` configuration that matches `gcloud iam workload-identity-pools create-cred-config` output for the same inputs (golden test), with `credential_source.file` pointing at the refreshed token.
- A project id where the project number is required fails with a named deploy-log line instead of minting for the wrong audience.
- On production, once the user's GCP gate is met, the service gets a `cloud-platform`-scoped access token through federation.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w2` (2026-11-17); the user filed all five proposed milestones with `/pm them all to w2` the same day. That reopens the 2026-08-23 deferral recorded in [`.pm/FUTURE-MAYBE.md`](../../../FUTURE-MAYBE.md) (from `w2/025`) under the Render-parity-first rule (AGENTS.md, user decisions 2026-10-17/2026-10-20; precedent: `w2/m252` absorbed a FUTURE-MAYBE item under the same rule). Ledger: [ADR018](../../../../docs/ADR018-render-parity.md) row "Managed OIDC for AI providers" (✖ on REST/GraphQL/MCP/UI, the last all-✖ row that is not an anti-goal) and [ADR030](../../../../docs/ADR030-pricing.md)'s pricing-parity table ("Open gap | OIDC authentication"). Render's page lists GCP beside AWS, Anthropic and OpenAI; ADR018's row omitted it until w2/m476 t001.
- **Goal linkage:** Render parity first (ADR008).
- **Expected outcome:** All four providers Render documents work on bex.
- **Why now:** With w2/m477's mechanism in place, GCP's only distinct piece is the generated credential configuration; finishing it closes the provider list.
- **Render parity:** Included: env-var behavior on every surface and the runtime variable compared with Render's page.
- **Depends on:** w2/m477 (delivery mechanism).
- **User gate:** t003 needs a GCP project; see the task.

## Park 2026-11-18

- **Shipped:** the GCP provider ([ADR116 D11](../../../../docs/ADR116-managed-oidc.md#d11--google-cloud-w2m479)).
  - `GCP_WORKLOAD_IDENTITY_PROVIDER` takes Render's resource name or full URL, with optional `GCP_SERVICE_ACCOUNT_EMAIL`. It mounts `/var/lib/render/oidc/gcp-token` for the provider's default audience `https://iam.googleapis.com/<resource name>`, beside `gcp-credentials.json`, which `GOOGLE_APPLICATION_CREDENTIALS` names.
  - `gcp-credentials.json` is byte for byte what `gcloud iam workload-identity-pools create-cred-config` writes for the same inputs (golden files).
  - A project id where the number belongs mounts nothing, and the deploy log names it.
  - `scripts/oidc-audience-verify.sh` checks GCP too.
- **Done:** t001, t002, t005, t006.
  - On dev-2 a Pro service with AWS and GCP got both files and gcloud's configuration byte for byte. A MinIO STS took the GCP token only for its own audience, and local gcloud reached Google's STS with the file. A project-id service went live with AWS only and the named deploy-log line ([t001](done/t001.md)).
  - `golang.org/x/oauth2/google` exchanges the file's token for `cloud-platform` and rereads it at each exchange ([t002](done/t002.md)).
- **Gate (user):** a GCP project with a workload identity pool and an OIDC provider trusting `https://oidc.bex.co/tea-daif693dqjvc73e7as3g` (default audience, `google.subject = assertion.sub`), plus a role for its principals, with the project number passed to the worker ([t003](t003.md) has the exact commands). bex holds no Google Cloud credential.
- **Gate (clears by itself):** a production pin carrying this milestone's commit and w2/m476's `68b45459b`. Check that `curl -sS https://oidc.bex.co/tea-daif693dqjvc73e7as3g/.well-known/openid-configuration` answers 200.
- **Then:** t003's production call, with its replies in this README. Next, t004's production walk of the GraphQL, MCP, env-group, dashboard and Blueprint writes, which dev-2 cannot make without OpenBao. Last, t007.
