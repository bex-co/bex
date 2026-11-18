# w2 · m477 — A service with `AWS_ROLE_ARN` gets a short-lived AWS identity token, as on Render's Managed OIDC

**Worker:** worker2 **Goal:** A Pro-or-higher service that sets `AWS_ROLE_ARN` gets, after its next deploy, a bex-signed token refreshed before its 60-minute expiry and named by `AWS_WEB_IDENTITY_TOKEN_FILE` in every container bex runs for it, so the AWS SDKs assume the role with no stored keys. This milestone builds the delivery mechanism the other providers reuse. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Design addendum: who mints, how the token reaches the pod, refresh, redeploy, below-Pro | 30m | w2/m476/t001 |
| t002 | Detect the trigger variable and gate on the workspace plan | 60m | t001 |
| t003 | Minter: issue and refresh each App's token before expiry | 90m | t002, w2/m476/t003 |
| t004 | Mount the token and set `AWS_WEB_IDENTITY_TOKEN_FILE` in every container bex runs for the service | 75m | t003 |
| t005 | In-cluster proof: an STS-compatible relying party exchanges the mounted token, across a refresh | 60m | t004, w2/m476/t006 |
| t006 | Production acceptance on real AWS (user gate: an AWS account trusting `oidc.bex.co/<tea>`) | 45m | t005 |
| t007 | Render parity | 20m | t006 |
| t008 | Simplify | 15m | t007 |
| t009 | Test coverage | 30m | t007 |
| t010 | Closeout | 10m | t008, t009 |

## Definition of done

- A service in a Pro workspace that sets `AWS_ROLE_ARN` (through REST, GraphQL, MCP, the dashboard, a Blueprint or a linked env group alike) has, after its next deploy, `AWS_WEB_IDENTITY_TOKEN_FILE` naming a read-only file with a bex-signed token (`aud` `sts.amazonaws.com`, Render's `sub`, `exp` = `iat` + 3600) in its instances, cron runs, pre-deploy Job and SSH sessions, and the file is replaced before expiry (observed across one refresh).
- That token gets credentials from an in-cluster STS-compatible relying party trusting `https://oidc.bex.co/<tea>` (`scripts/oidc-aws-verify.sh` exits 0) and, once the user's AWS gate is met, from real AWS STS; a trust policy pinning another service's `sub` refuses it.
- A Hobby-workspace service with `AWS_ROLE_ARN` keeps the variable, gets no token, and its deploy log says why; a service without a trigger variable has a byte-identical pod spec.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w2` (2026-11-17); the user filed all five proposed milestones with `/pm them all to w2` the same day. That reopens the 2026-08-23 deferral recorded in [`.pm/FUTURE-MAYBE.md`](../../FUTURE-MAYBE.md) (from `w2/025`) under the Render-parity-first rule (AGENTS.md, user decisions 2026-10-17/2026-10-20; precedent: `w2/m252` absorbed a FUTURE-MAYBE item under the same rule). Ledger: [ADR018](../../../docs/ADR018-render-parity.md) row "Managed OIDC for AI providers" (✖ on REST/GraphQL/MCP/UI, the last all-✖ row that is not an anti-goal) and [ADR030](../../../docs/ADR030-pricing.md)'s pricing-parity table ("Open gap | OIDC authentication").
- **Goal linkage:** Render parity first (ADR008); services stop storing long-lived cloud keys.
- **Expected outcome:** An app configured for Render's AWS OIDC moves to bex unchanged: only the issuer its AWS role trusts changes.
- **Why now:** The delivery mechanism here is what w2/m478–m480 reuse; AWS goes first because an STS-compatible relying party can prove it in-cluster without waiting on external accounts.
- **Render parity:** Included: tenant-facing behavior of an env var set through REST, GraphQL, MCP, the dashboard, Blueprints and the CLI, the plan gate's wording on each surface, and runtime files compared with Render's page.
- **Depends on:** w2/m476 (issuer and signer).
- **User gate:** t006 needs an AWS account; see the task.
- **Brainstorm recommendations, not user decisions** (record the chosen answer and its basis in the ADR): D2 issuer host `oidc.<root domain>` (`oidc.bex.co` in production, configurable for self-hosters); D3 deliver the token through a per-App Secret that bex-api refreshes and the pod mounts read-only (kubelet keeps Secret volumes in tmpfs; residual: a ≤60-minute token sits in etcd), with memory-only CSI delivery filed to FUTURE-MAYBE as the stricter variant; D4 below Pro the trigger variable is accepted and kept, no token is mounted, and the deploy log says why (Render's Hobby behavior is **unverified**); D5 one shared ES256 P-256 key set in a `bex-system` Secret, not OpenBao transit, so token refresh does not depend on OpenBao's manual unseal.
