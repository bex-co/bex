# docs/CLAUDE.md

Docs live here — one file per topic. Root [CLAUDE.md](../CLAUDE.md) points here for the full catalog (cascading: this file loads only when working in `docs/`).

## How to use

- Each `ADR*.md` is self-contained; read the ADR directly for design + status.
- Render-parity ledgers ([ADR018](ADR018-render-parity.md), [cli-compatibility-checklist.md](cli-compatibility-checklist.md)) are evidence-backed.
- Security reviews form a chain ADR028 → ADR084 (see § Security lineage); read the latest for current residual.

## Catalog — by topic

### Vision, architecture, platform

- [ADR008-vision.md](ADR008-vision.md) — mission, AI-native pillars, roadmap
- [ADR002-architecture.md](ADR002-architecture.md) — self-managed cluster, two layers, node pools
- [ADR001-go-and-gitops.md](ADR001-go-and-gitops.md) — why product (Go) ≠ GitOps (platform infra)
- [ADR003-control-plane.md](ADR003-control-plane.md) — Postgres source of truth vs operator mechanism
- [ADR019-infra-credentials.md](ADR019-infra-credentials.md) — bootstrap credential inventory + trust chain
- [ADR053-node-instance-types.md](ADR053-node-instance-types.md) — Hetzner cx33/cpx32 policy

### API, parity, Blueprint

- [ADR006-bex-api.md](ADR006-bex-api.md) — REST/GraphQL/MCP: one core, thin adapters, Render compat
- [ADR018-render-parity.md](ADR018-render-parity.md) — parity ledger (capability × surface)
- [cli-compatibility-checklist.md](cli-compatibility-checklist.md) — Render CLI compat matrix
- [ADR049-render-yaml-parity.md](ADR049-render-yaml-parity.md) — `render.yaml` canonical Blueprint contract
- [ADR020-identifiers.md](ADR020-identifiers.md) — typed ids `<prefix>-<xid>` (`tea-/srv-/cdm-`)
- [ADR032-environments.md](ADR032-environments.md) — named env subsets of a Project

### Auth, members, GitHub

- [ADR012-auth.md](ADR012-auth.md) — Ory Kratos + Hydra on CNPG; OAuth 2.1
- [ADR027-sso.md](ADR027-sso.md) — social OIDC via Kratos; enterprise SAML deferred
- [ADR024-members.md](ADR024-members.md) — workspace members & roles, OpenFGA tuples
- [ADR026-github-integration.md](ADR026-github-integration.md) — self-hosted GitHub App, deploy keys
- [ADR078-github-workspace-connections.md](ADR078-github-workspace-connections.md) — N installations per workspace
- [ADR025-connect-an-agent.md](ADR025-connect-an-agent.md) — Claude/Cursor → `/mcp` over OAuth 2.1

### App lifecycle & hosting types

- [ADR004-app-deployment.md](ADR004-app-deployment.md) — deploy flow, health gating, revisions
- [ADR005-custom-domain.md](ADR005-custom-domain.md) — `spec.hosts[]`, Traefik + cert-manager
- [ADR007-restart-suspend-and-resume.md](ADR007-restart-suspend-and-resume.md) — lifecycle verbs
- [ADR029-static-sites.md](ADR029-static-sites.md) — `static_site` → object-store origin + static-server
- [ADR038-cron-jobs.md](ADR038-cron-jobs.md) — `cron_job` → CronJob, runAt/cancelRun
- [ADR041-service-addresses.md](ADR041-service-addresses.md) — internal `slug:port` + public `slug.domain`
- [ADR034-scalable-build-pipeline.md](ADR034-scalable-build-pipeline.md) — BuildKit workers, concurrency
- [ADR060-build-worker-reliability-and-performance.md](ADR060-build-worker-reliability-and-performance.md) — D1–D8 reliability/perf
- [ADR009-postgresql-management.md](ADR009-postgresql-management.md) — `Database` CR → CNPG
- [ADR021-keyvalue-management.md](ADR021-keyvalue-management.md) — `KeyValue` CR → Valkey
- [ADR082-persistent-disks.md](ADR082-persistent-disks.md) — `spec.disk` → Hetzner volume PVC, $0.175/GB-mo, min 10 GB (reverses the stateless-first non-goal); arm its snapshots with [runbooks/disk-snapshot-setup.md](runbooks/disk-snapshot-setup.md)

### Sandboxes, agents, mobile, SSH

- [ADR014-sandboxes.md](ADR014-sandboxes.md) — E2B-compatible sandboxes over opensandbox
- [ADR042-sandbox-cluster-substrate.md](ADR042-sandbox-cluster-substrate.md) — multi-node OpenSandbox substrate, Kata/gVisor
- [ADR044-sandbox-runtime-comparison.md](ADR044-sandbox-runtime-comparison.md) — AgentENV vs OpenSandbox survey
- [ADR035-ssh.md](ADR035-ssh.md) — instance SSH, gateway + Kubernetes exec bridge
- [ADR054-open-in-zed.md](ADR054-open-in-zed.md) — `zed://ssh/ags-…@ssh.bex.co/workspace`
- [ADR047-cloud-coding-agent-sessions.md](ADR047-cloud-coding-agent-sessions.md) — sandbox-per-session, ACP driver, gateway grants
- [ADR051-agent-session-transcript.md](ADR051-agent-session-transcript.md) — durable conversation history (headless recorder)
- [ADR059-agent-sandbox-hibernation.md](ADR059-agent-sandbox-hibernation.md) — Active/Hibernated/Deleted + object-storage snapshots
- [ADR062-sandbox-credential-vault.md](ADR062-sandbox-credential-vault.md) — BYO key via gateway proxy (mandatory)
- [ADR065-agent-session-archive.md](ADR065-agent-session-archive.md) — archive/unarchive/delete, replay-only tickets
- [ADR048-mobile.md](ADR048-mobile.md) — supervision-first PWA + push, phone as mission control
- [bex-cli.md](bex-cli.md) — CLI launcher release train (`bex-cli/v*`)

### Observability, metering, billing, notifications

- [ADR010-observability.md](ADR010-observability.md) — logs (query + live-tail) + metrics
- [ADR023-usage-metering.md](ADR023-usage-metering.md) — hourly rollup + `GET /v1/usage`
- [ADR040-billing-metronome.md](ADR040-billing-metronome.md) — Stripe Billing: `usage_hourly` → Customers/Subscriptions, `BEX_STRIPE_SECRET_KEY`
- [ADR030-pricing.md](ADR030-pricing.md) — price sheet + `estimatedCost` (advisory; invoices = ADR040)
- [ADR046-payment-onboarding-and-paid-gating.md](ADR046-payment-onboarding-and-paid-gating.md) — JIT payment-method gate (`BEX_REQUIRE_PAYMENT_METHOD`)
- [ADR071-tenant-billing-credits.md](ADR071-tenant-billing-credits.md) — Stripe Credit Grants
- [ADR052-notifications.md](ADR052-notifications.md) — deploy + audit → email / webhooks / push
- [ADR075-user-onboarding.md](ADR075-user-onboarding.md) — first-run checklist, card wall, demo-mode deferral

### Secrets, backups, isolation

- [ADR013-secrets.md](ADR013-secrets.md) — OpenBao tenant credentials, k8s auth `tenants/*`
- [ADR016-sealed-secrets.md](ADR016-sealed-secrets.md) — SealedSecrets for infra creds
- [ADR011-etcd-backup-restore.md](ADR011-etcd-backup-restore.md) — etcd snapshot → object storage
- [ADR015-openbao-backup-restore.md](ADR015-openbao-backup-restore.md) — OpenBao Raft snapshot
- [ADR031-platform-data-backup.md](ADR031-platform-data-backup.md) — consolidated backup policy
- [ADR050-encrypted-platform-backups.md](ADR050-encrypted-platform-backups.md) — age-encrypted backups, per-store creds
- [ADR036-ca-rotation-runbook.md](ADR036-ca-rotation-runbook.md) — K8s CA + admin-cert rotation
- [ADR037-openbao-rekey-runbook.md](ADR037-openbao-rekey-runbook.md) — OpenBao root-token / Shamir rekey
- [ADR022-tenant-isolation.md](ADR022-tenant-isolation.md) — east-west NetworkPolicy (superseded by ADR043 for boundary)
- [ADR043-tenant-namespace-isolation.md](ADR043-tenant-namespace-isolation.md) — `workspace = namespace` isolation
- [ADR074-workspace-scoped-artifact-identity.md](ADR074-workspace-scoped-artifact-identity.md) — workspace-scoped Zot repos + S3 prefixes (`W/A`)

### Deploy-from-chat, workflows, evals

- [ADR017-deploy-from-chat.md](ADR017-deploy-from-chat.md) — deploy-from-chat via `Core.Create` + HMAC webhook
- [ADR033-workflows.md](ADR033-workflows.md) — Temporal orchestration (proposed, off-roadmap)
- [ADR070-openchoreo-evaluation.md](ADR070-openchoreo-evaluation.md) — reject OpenChoreo whole-platform adoption
- [ADR039-operator-audit-and-platform-reuse.md](ADR039-operator-audit-and-platform-reuse.md) — operator audit + reuse candidates
- [ADR058-release-engineering.md](ADR058-release-engineering.md) — `bex/vX.Y.Z` lockstep, CLI `bex-cli/v0.x` until 1.0

### Security review lineage (ADR028 → round 22)

Each entry is a codex-security triage; earlier rounds' residuals are re-confirmed in later ones. Read the latest ([ADR084](ADR084-security-review-round22.md)) for current posture.

- [ADR081-security-review-harness-glm.md](ADR081-security-review-harness-glm.md) — harness architecture (proposed): reuse codex-security with a GLM model if possible, else mirror it on GLM 5.2; one harness at a time
- [ADR028-security-review.md](ADR028-security-review.md) — round 0 audit (evidence-backed)
- [ADR045-security-review-round3.md](ADR045-security-review-round3.md) — round 3: host-hijack fix
- [ADR055-security-review-round4.md](ADR055-security-review-round4.md) — round 4: 12 findings, 6 fixed (F1/F4/F5/F8/F11/F12)
- [ADR056-security-review-round5.md](ADR056-security-review-round5.md) — round 5: 17 findings, 13 fixed
- [ADR057-security-review-round6.md](ADR057-security-review-round6.md) — round 6: 16 findings, 10 fixed
- [ADR072-security-review-round7.md](ADR072-security-review-round7.md) — round 7: 10 findings, 7 fixed (first zero highs)
- [ADR061-security-review-round8.md](ADR061-security-review-round8.md) — round 8: 12 findings, 5 fixed
- [ADR062-sandbox-credential-vault.md](ADR062-sandbox-credential-vault.md) — (hardened by ADR064, see above)
- [ADR063-security-review-round9.md](ADR063-security-review-round9.md) — round 9: 13 findings, 9 fixed
- [ADR064-security-review-round10.md](ADR064-security-review-round10.md) — round 10: 20 findings
- [ADR066-security-review-round11.md](ADR066-security-review-round11.md) — round 11: 10 findings, 7 fixed
- [ADR067-security-review-round12.md](ADR067-security-review-round12.md) — round 12: 10 findings, 7 fixed
- [ADR068-security-review-round13.md](ADR068-security-review-round13.md) — round 13: 9 findings, 7 fixed (sandbox-exec side door closed)
- [ADR069-security-review-round14.md](ADR069-security-review-round14.md) — round 14: 6 findings, auth-layer `bex.api` scope fix
- [ADR073-security-review-round15.md](ADR073-security-review-round15.md) — round 15: 8 findings, 5 fixed
- [ADR076-security-review-round16.md](ADR076-security-review-round16.md) — round 16: 13 findings, 12 fixed
- [ADR077-security-review-round17.md](ADR077-security-review-round17.md) — round 17: 15 findings, 11 fixed
- [ADR079-security-review-round18.md](ADR079-security-review-round18.md) — round 18: 10 findings (1 crit), 8 fixed
- [ADR080-security-review-round19.md](ADR080-security-review-round19.md) — round 19: 8 findings, 7 fixed (CI env gates, secrets CAS, metrics scoping)
- [ADR083-security-review-round20.md](ADR083-security-review-round20.md) — round 20: self-hosted CI runner risks accepted (persistent hosts, shared pool, fork-PR policy)
- [ADR084-security-review-round22.md](ADR084-security-review-round22.md) — round 22: dnR05P triage; 3 security fixes, 5 boundary/correctness hardenings, 2 accepted residuals

Other docs: [PRFAQ001-bex-v1-hosting.md](PRFAQ001-bex-v1-hosting.md), [assets/](assets/) media.
