# w4 · m5 — Platform secrets: OpenBao on the cluster (+ ADR)

**Worker:** worker4 **Goal:** a versioned, policy-scoped secret store for _tenant_ credentials runs on the cluster as platform infrastructure (mirroring the w4/m1 Ory playbook: ADR → pinned Argo chart → out-of-band bootstrap → verify script), with the bex-api ServiceAccount able to read/write only `tenants/*` — ready for the product wiring in m6. Distinct from w1/m7 t003 (platform deploy secrets at rest in git). **Status:** done (2026-07-07; E2E-verified on the local mock cluster — init/unseal idempotency, KV v2 write/read, scoped `bex-api` SA login via the Kubernetes auth method, and restart durability. Note: OpenBao's pod needs pinning to the control-plane node on this CAPD cluster — `service_registration "kubernetes"` calls the apiserver directly, and worker nodes can't reach it here, so a worker-scheduled pod comes up `Running` but hangs on every request forever; see docs/secrets.md's local-CAPD quirks.)

## Tasks (in order)

| id   | title                                                                                                            | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | ADR `docs/secrets.md`: OpenBao for tenant credentials — alternatives, storage backend, unseal strategy — **DONE** | 35m | —          |
| t002 | Argo Application for OpenBao (pinned chart, base values + local overlay) — **DONE**                              | 35m | t001       |
| t003 | `scripts/bao-init.sh` — out-of-band init/unseal via `.env` + KV v2 mount `tenants/` — **DONE**                    | 40m | t002       |
| t004 | Kubernetes auth method: role + policy for the bex-api ServiceAccount scoped to `tenants/*` — **DONE**             | 30m | t003       |
| t005 | `scripts/bao-verify.sh` — KV write/read, scoped SA login, restart durability, sealed-state behavior — **DONE**    | 35m | t004       |
| t006 | Docs: index entry, env tables, prod deploy path — **DONE**                                                        | 25m | t005       |
| t007 | Simplify — run `/simplify` over the code this milestone changed — **DONE**                                        | 20m | t006       |
| t008 | Test coverage — meaningful tests for the behavior this milestone shipped — **DONE**                               | 30m | t006       |

## Definition of done

On the local mock cluster: an OpenBao pod is healthy via Argo (pinned chart, values in git, no secret material in git); `scripts/bao-init.sh` initializes and unseals it idempotently with unseal keys + root token held only in `.env` (names mirrored to `.env.example`/`.env.template`); the KV v2 mount `tenants/` exists; the bex-api ServiceAccount logs in via the Kubernetes auth method and can read/write under `tenants/*` but cannot read `sys`; `scripts/bao-verify.sh` exits 0, including after `kubectl rollout restart` (state survives the pod); the ADR is merged with the storage-backend and unseal decisions recorded.

## Source + Goal linkage

- **Source:** /pm-brainstorm 2026-07-06 (user request: "add openbao so tenants can store their credentials").
- **Goal linkage:** roadmap #1 (multi-tenant control plane) and pillar 4 (deploy-from-chat) — an agent deploying an app must hand over that app's credentials through an API, not bake them into images or paste them into etcd-plaintext CRs.
- **Expected outcome:** a versioned, policy-scoped secret store runs on the cluster as platform infra, distinct from w1/m7's git-secrets-at-rest work; bex components authenticate to it with their own ServiceAccounts.
- **Why now:** m6 (product wiring) and w1/m2's tenants tables both need a place for credentials to live _before_ they exist; deploying the substrate is independent of the in-flight m4 (OpenFGA) so it can proceed in parallel, reusing the still-fresh m1 playbook.
