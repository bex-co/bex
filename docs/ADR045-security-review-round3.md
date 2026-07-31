# Security review round 3: billing plane · static plane · tenant-namespace plane

**Status:** implemented (w7/m57) · **Scope:** an evidence-backed adversarial audit of the surfaces shipped in the window **2026-07-20 → HEAD** — the Stripe billing lifecycle, the static-site serving/publish plane, the per-tenant-namespace plane, and the cross-cutting secret/RBAC/CI-guard posture. Third pass in the ADR028 → w1/m53 → this lineage; continues `GOAL.md` #7.

## Scope map

In-window surfaces and their audit disposition (double-coverage of already-verified work is avoided by naming each exclusion's owner):

| Surface | Owner | Audited? |
| --- | --- | --- |
| Stripe billing lifecycle (webhook/enforce/outbox) | w7/m50–m53 | **IN** (this ADR §1) |
| Static serving/publish residuals | w7/m54 | **IN** beyond m54's owned items (§2) |
| Per-tenant namespace plane (quota/prune/labels) | w3/m31 | **IN** beyond m35/t013 admission (§3) |
| Cross-cutting secrets + RBAC/NetworkPolicy/CI guard | (spans) | **IN** (§4) |
| Sandbox create/list/exec + runtime hardening | w3/m32/33/35 | **EXCLUDED** — m35 closed + prod-verified |
| Cross-workspace authz matrix | w7/m55 | **EXCLUDED** — shipped same session |
| SSH-gateway least-privilege DB role | w7/m56 | **EXCLUDED** — shipped same session |
| Static alias authority / browser divergence / PSL | w7/m54 | **EXCLUDED** — owner-decided (PSL waived) |

## Severity summary

| # | Area | Finding | Severity | Disposition |
| --- | --- | --- | --- | --- |
| 1 | Static | Cross-tenant host claim / traffic hijack unguarded on create/blueprint/deploy-manifest | **High** | **Fixed in place** |
| 2 | Billing | Seal horizon vs. usage catch-up window decoupled — sub-48h seal lets an exported row be rewritten | Medium | **Fixed in place** |
| 3 | Cross-cutting | `.env.example` omits 6 in-window Stripe vars (repo mirror-rule violation) | Medium | **Fixed in place** |
| 4 | Tenant-ns | ResourceQuota omits storage/PVC/LoadBalancer caps (resource-exhaustion axis) | Medium | **Fixed in place** (w7/m59) |
| 5 | Cross-cutting | No CI guard forbids a ClusterRoleBinding to secret-bearing `bex-tenant-api`/`-operator` | Low | **Fixed in place** |
| 6 | Tenant-ns | Toggling `BEX_TENANT_SANDBOX_NAMESPACES` off prunes live `<ws>-sandbox` namespaces | Low | **Fixed in place** (w7/m62) |
| 7 | Static | Publish Job holds the wildcard `bex-builder` pull credential | Low | **Already resolved** (w3/m35); verified w7/m62 |
| 8 | Cross-cutting | No completeness guard forces new directly-mounted (outside-gate) routes to be classified | Low | Follow-up |
| 9 | Tenant-ns | bex-api gains cluster-wide namespace/networkpolicy/rolebinding authority | Medium→mitigated | Accepted risk |
| 10 | Tenant-ns | LimitRange sets no per-container Min / per-Pod Max | Low | Accepted risk |

No critical findings. Every plane was probed with sweep evidence; the clean results (billing webhook/enforce/secret custody, static S3 split, namespace prune-safety/label-spoof/default-deny) are recorded below with what was checked.

## 1. Billing plane (w7/m50–m53)

**Clean (evidence).** The public webhook `POST /v1/webhooks/stripe` verifies the `Stripe-Signature` HMAC before any state change, fails closed (503) when the secret is unset, enforces Stripe's 300s timestamp tolerance, dedups by `PRIMARY KEY (event_id)` + `ON CONFLICT DO NOTHING`, and orders by `provider_created_at` so a stale event never overrides a newer applied state (`internal/billing/webhook.go`, `internal/store/billing_lifecycle.go:255–298`). Every enforce/recover transition writes its `audit_events` row **in the same transaction** as the status flip and is a machine action gated on owed-ness (excluded/comped skipped); human overrides route only through the `BEX_CP_TOKEN`-guarded internal API with a required actor+reason (`internal/store/billing_lifecycle.go:454–499`, `internal/billing/enforcement.go:127–230`). No Stripe secret is logged, returned in an error, or stored in cleartext — grep of the billing+store packages for secret handling returned zero leak sites; migrations 0051/0052 store only provider ids + state (`0052_*.up.sql` header). Tenant usage quantity is server-derived (Prometheus/Traefik/build-Job), never tenant-supplied; meter events are per-workspace customer-scoped with a deterministic per-row transaction id; the epoch/34-day clamp cannot be walked forward.

**Finding 2 [Medium] — Seal horizon and rollup catch-up window were coupled only by convention.** The emitter exports a `usage_hourly` row once older than `SealHours` (`SelectUnemittedUsage`), while the usage rollup rewrites `quantity` for any window within `catchupLimit = 48h` (`UpsertUsageHourly` `DO UPDATE`, no `billing_export_state` guard). Safety held only because `DefaultSealHours (48h) == catchupLimit (48h)` — two constants in two packages with no code coupling. `BEX_STRIPE_SEAL_HOURS=24` would export a row still inside the rewrite window; a later corrected quantity overwrites the emitted row and is never re-shipped. **Fixed in place:** `usage.CatchupWindow` is now exported and `cmd/api` clamps the seal horizon up to it via `billing.ClampSealHours`, making the export and rewrite windows non-overlapping by construction (`internal/billing/emitter.go`, `internal/usage/service.go:295`, `cmd/api/main.go`). Regression + defaults-invariant tests: `internal/billing/sealclamp_test.go`.

## 2. Static plane (w7/m54 residuals)

**Clean (evidence).** The reader (`bex-static-read-s3`) and publisher (`bex-static-publish-s3`) are genuinely separate least-privilege Wasabi identities: reader = `GetObject`/`ListBucket` on `bex-static` only; publisher adds `PutObject`/`DeleteObject`, still `bex-static`-scoped, never `bex-tfstate`; `scripts/static-s3-credentials.sh` refuses any other bucket, refuses static==tfstate, and rejects any extra attached policy, with a positive+negative verify matrix (`infra/wasabi/static-s3-*.json`). Publish/purge object keys are `s3://<bucket>/<app.Name>/…` where `app.Name = CRName(tenant,name)` is globally distinct, so no cross-tenant prefix write; `--no-follow-symlinks` + `..`-rejection block source escape (`publish.go`).

**Finding 1 [High] — Cross-tenant host claim / traffic hijack unguarded on the create paths.** The `reservedHost` + `hostClaimedElsewhere` gate (w7/m6) ran **only** inside `AddDomain`. Every create path — REST/GraphQL/MCP create (`specFromCreate`), blueprint upsert, and deploy manifest (`applyCreateToSpec`) — bound `req.Hosts` straight onto `spec.Host`/`spec.Hosts` with no check, and the store `CreateApp` never wrote the `domains` row so the `domains.host UNIQUE` index never fired. A tenant could create a service (or blueprint) claiming `api.bex.co`, `dashboard.bex.co`, `<victim>.onbex.co`, or another tenant's existing custom domain, and the operator would mint the Ingress → shadow/hijack. **Fixed in place:** the same gate now runs on the single shared create write seam `writeNewApp` via `ensureHostsClaimable`, covering all three create paths at one point; a blueprint re-apply of the App's own hosts is not self-rejected (`hostClaimedElsewhere` skips the App's own name) (`internal/apps/domains.go`, `internal/apps/service.go:1789`). Regression test: `internal/apps/crosstenant_host_test.go` (reserved + claimed rejected; free host still creates).

**Finding 7 [Low] — Publish Job holds the wildcard `bex-builder` pull credential.** When the publish Job runs in a separate `BEX_BUILD_NAMESPACE`, its `PullSecret` resolves to `bex-build/bex-registry-pull` (the `bex-builder` identity with wildcard-repo read/write), not a repo-scoped credential. Not a live break — the pod only ever pulls the operator-resolved artifact for its own App, and the tenant cannot read the mounted dockerconfig — but it is an over-broad credential in a short-lived platform pod. **Already resolved (verified w7/m62):** the audit snapshot predated the fix landed in `7e05d80f` (w3/m35). `buildJobPullSecret` now returns the repo-scoped per-App `reg-pull-<name>` whenever `BEX_REGISTRY_NS` is set — which production is (`lego/operator/config/manager/manager.yaml`) — mirrored into the build namespace intact for the publish pod by `copyBuildRegistryCredential` (`build_registry_secret.go`); the wildcard `bex-registry-pull` is only the fallback for per-App-disabled (dev) deployments, the m8 shared-credential design. Both branches are pinned by `TestBuildJobPullSecret` (`publish_pull_secret_test.go`). The delete-time purge Job still passes `RegistryBuildPullSecret`, but it pulls the platform AWS-CLI tool image (not a tenant artifact) which the repo-scoped `reg-pull-<name>` cannot fetch — so its broader read is required there, not a tenant over-grant.

## 3. Tenant-namespace plane (w3/m31, beyond m35/t013)

**Clean (evidence).** Prune-on-delete lists namespaces by a two-label AND gate (`managed-by=bex-controlplane` AND non-empty `workspace`) and a `ListTenants` failure returns before prune — platform namespaces carry neither label and tenant ids are `tea-<xid>` (uncollidable), so prune can never select a platform namespace (`TestPruneNeverTouchesUnmanaged`). Identity labels the NetworkPolicy/quota select on live on the **namespace object** (tenant-unwritable) and the policies use an empty PodSelector (= all pods), so a tenant workload cannot escape default-deny or quota by forging pod labels. A fresh `<ws>` gets a both-directions default-deny NetworkPolicy applied before any workload can schedule (`namespaces.go:162–204,613–629`, `TestReconcileProvisionsHostingNamespaceWithBaseObjects`).

**Finding 4 [Medium] — ResourceQuota omits storage/PVC/LoadBalancer caps.** `quotaForPlan` caps cpu/memory/pods/jobs and `count/{apps,databases,keyvalues}` but has no `requests.storage`, `persistentvolumeclaims`, `ephemeral-storage`, or `services.loadbalancers`/`nodeports` (`store/namespaces.go:363–383`). Each managed Database/KeyValue can request arbitrarily large (autoscaling) PVCs, and nothing bounds aggregate storage or billable cloud LBs per namespace. This becomes load-bearing exactly when w3/m34 makes this quota the **sole** enforcement path. **Fixed in place (w7/m59):** `quotaForPlan` now carries `requests.storage` (free 20Gi / paid 5Ti), `persistentvolumeclaims` (4 / 200), and `services.loadbalancers`/`services.nodeports` (0 for all plans), documented in [ADR043 §D3](ADR043-tenant-namespace-isolation.md#d3--zero-trust-east-west-enforcement-pushed-to-the-lowest-layer). A quota-blocked grow is surfaced observably and self-clears: the Postgres disk-autoscaler pre-flights headroom and sets a `DiskGrowthBlockedByQuota` condition (it patches the CNPG Cluster, not the PVC, so a rejection would otherwise strand `spec.storageGB`), while the KeyValue reconciler catches the `exceeded quota` Forbidden on its own PVC resize and surfaces `StorageBlockedByQuota` instead of hot-looping. The `ephemeral-storage` dimension and LimitRange Min/Max (Finding 10) remain accepted risk.

**Finding 6 [Low] — Sandbox-toggle prune reaps live namespaces.** With `BEX_TENANT_SANDBOX_NAMESPACES` off, `ReconcileOnce` omits `<ws>-sandbox` from the desired set, so `pruneOrphans` deletes a still-live sandbox namespace (keying on config state, not workspace existence). Operator-controlled + monotonic in practice; filed as a follow-up. **Fixed in place (w7/m62):** `ReconcileOnce` now always marks a live workspace's `<ws>-sandbox` as desired (the toggle governs only create/converge), so a prune reaps it only when the workspace itself is gone; regression-tested (`TestSandboxNamespaceSurvivesToggleOffButPrunesOnWorkspaceDelete`).

**Finding 10 [Low] — LimitRange has no Min / per-Pod Max.** Aggregate quota still bounds the blast radius; accepted risk (optional hardening in m34).

## 4. Cross-cutting (secrets · RBAC · CI guards)

**Clean (evidence).** Every in-window secret (Stripe key/webhook, sandbox-exec HMAC, static S3 read/publish) uses an out-of-band installer script that never prints or commits material and never renders into GitOps; none reaches logs, `audit_events`, DB cleartext, or error strings. The Stripe webhook route, sandbox-exec listener, and namespace-reconciler RBAC are each CI-guard-covered (`TestStripeWebhookBypassesAuthGate`, `sandbox_exec_test.go`, `gitops-validate.sh:1170–1227`). NetworkPolicy posture over the window is clean (no unguarded hole; sandbox-exec `:8081` governed by a guarded Cilium deny-complement).

**Finding 3 [Medium] — `.env.example` omitted 6 in-window Stripe vars** (`BEX_STRIPE_TAX_CODE`, `_TAX_BEHAVIOR`, `_PORTAL_CONFIGURATION_ID`, `_DUNNING_ENABLED`, `_GRACE_PERIOD`, `_RECONCILE_INTERVAL`) — all read from `.env` by `scripts/stripe-billing-secret.sh`, violating the repo mirror rule. **Fixed in place:** the six value-less lines added to `.env.example`.

**Finding 5 [Low] — No CI guard forbade a ClusterRoleBinding to the secret-bearing `bex-tenant-api`/`bex-tenant-operator` ClusterRoles** (cluster-wide `secrets` write, safe only when bound per-namespace). No such binding exists today, but nothing prevented one. **Fixed in place:** `scripts/gitops-validate.sh` now fails on a ClusterRoleBinding to either role (mirroring the sandbox-role guard).

**Finding 8 [Low] — No completeness guard forces new directly-mounted (outside-gate) routes to be classified.** `TestCrossWorkspaceRESTMatrix` enumerates only feature-registered routes; the directly-mounted public routes in `server.go`'s `Handler()` each rely on a bespoke `*BypassesAuthGate` test. Filed as a follow-up (add a completeness guard over the composed `Handler()` mux).

**Finding 9 [Medium→mitigated] — bex-api's cluster-wide namespace/networkpolicy/rolebinding authority** (ADR043 NamespaceReconciler) is a genuine expansion, but is intentional and thoroughly mitigated: a `Fail`-policy ValidatingAdmissionPolicy pins writes to the `bex-api` SA + `tea-`/`-sandbox` regex + name allowlists, the reconciler creates only namespaced RoleBindings, and `gitops-validate.sh:1170–1227` asserts the whole shape. Accepted risk.

## Follow-up register

| Finding | Severity | Owner / cross-ref |
| --- | --- | --- |
| 4 — quota storage/PVC/LB caps | Medium | **Fixed in place (w7/m59)**; w3/m34 later makes the quota the sole enforcement path |
| 7 — repo-scope the publish-Job pull credential | Low | **Already resolved (w3/m35, `7e05d80f`)**; verified + documented w7/m62 |
| 6 — sandbox-toggle prune reaps live namespaces | Low | **Fixed in place (w7/m62)** (was note `w7/011`) |
| 8 — completeness guard for outside-gate routes | Low | new note `w7/012` |

## Out of scope

Sandbox TCB (w3/007/008), the static PSL decision (owner-waived, w7/m54), and the cross-workspace authz matrix + gateway DB role (w7/m55/m56, shipped this session) — each owned and evidenced elsewhere; re-auditing would be double-coverage.
