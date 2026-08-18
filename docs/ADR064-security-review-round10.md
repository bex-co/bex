# ADR064: Security review round 10 (codex-security repo scan)

- **Status:** Accepted (remediation in place)
- **Date:** 2026-08-16
- **Scan:** codex-security `bex/codex-security-bex-3w3XZc` (standard static review at revision `5ee9e8`; 20 findings — 3 high, 9 medium, 8 low, all high confidence)
- **Lineage:** tenth pass in the ADR028 → ADR045 → ADR055 → ADR056 → ADR057 → ADR072 → ADR061 → ADR063 lineage.

## Summary

| # | Finding | Severity | Disposition |
| --- | --- | --- | --- |
| 1 | Native builds receive runtime secrets | high | **Rejected as a distinct privilege escalation** — the same tracked revision is deployed with the same runtime environment |
| 2 | Caller-selected model endpoint receives reusable key | high | **Fixed** — agent profile resolves to a platform-registered provider origin |
| 3 | Shipped manifests use direct model-key injection | high | **Fixed** — proxy mandatory, manifests wired, sandbox seam admits placeholders only |
| 4 | Unbounded log resource fan-out | medium | **Fixed** — raw cap 20 + first-seen dedup before any authorization/upstream work |
| 5 | Mutations consume stale positive authorization | medium | **Fixed** — all write relations use `CheckFresh` centrally |
| 6 | Unlimited active push-device registrations | medium | **Fixed** — race-safe subject/workspace quotas (10/1000) |
| 7 | GET GraphQL bypasses body cap | medium | **Fixed** — GraphQL is mounted POST-only |
| 8 | `onbex.co` lacks browser public-suffix isolation | medium | **Accepted external blocker** — seventh repeat; shared hosting needs PSL eligibility or a domain restructure |
| 9 | Unbounded metrics output filters | medium | **Fixed** — unique list capped at eight before authorization/upstream work |
| 10 | Model proxy permits arbitrary provider operations | medium | **Fixed** — provider-specific JSON POST inference allowlists |
| 11 | Model proxy bodies/streams/concurrency unbounded | medium | **Fixed** — 4 MiB request, 64 MiB response, 32/2 concurrency, 2m read and 2h total limits |
| 12 | Registration retains prior Apollo account data | medium | **Fixed** — successful registration clears the singleton cache before session transition |
| 13 | Metrics Server skips kubelet certificate verification | low | Deferred — fourth repeat; requires kubelet serving certificates/CSR approval |
| 14 | Ory admin APIs reachable from arbitrary platform pods | low | **Fixed for network reachability** — auth default deny + exact source/port policies |
| 15 | Operator destructive RBAC exceeds admission containment | low | **Fixed** — DELETE, PVC, kpack-build, and status/finalizer operations confined to canonical namespaces |
| 16 | Cached model key survives terminal validation | low | **Fixed** — no credential cache; every exchange re-mints against current lifecycle |
| 17 | Barman controller can read/delete every Secret | low | **Fixed** — unused upstream Secret ClusterRole rule removed |
| 18 | DNS registry names permit blind node-origin probes | low | Accepted constrained residual — repeat of ADR060/ADR061 |
| 19 | Privileged inputs are mutable or unverified | low | **Fixed for every cited effective path** — checksums and image digests pinned; dashboard production was already digest-overridden |
| 20 | Deploy-hook token appears in ingress request path | low | Accepted custody residual — repeat of ADR072 #4 |

Fifteen findings changed code or deployment controls. One was not a distinct capability, and four are standing operational/external residuals with unchanged prerequisites.

## 1 — Native build runtime environment (high): rejected as a distinct capability

The dataflow is real: a native build command can read the same environment that the service receives at runtime. The claimed attacker, however, controls the tracked auto-deploy revision. A malicious revision that builds successfully is immediately executed as the service with that complete runtime environment and public network access. Removing the values from the build shell therefore does not prevent that repository contributor from reading or exfiltrating them; it only moves the identical action to process startup and breaks Render-compatible build commands that legitimately need configuration.

This is not an argument that builds are trusted. They stay inside the isolated execution boundary, receive no platform credentials, and use BuildKit secret mounts so values do not persist in layers. A future separately-scoped build-secret product can improve least privilege for operators who intentionally deploy code that should not receive runtime credentials, but it does not remediate the threat stated by this finding.

## 2, 3, 10, 11, 16 — model credential boundary: fixed

The optional generic pass-through became a mandatory, registered, bounded broker:

- `RegisteredModelEndpoint` maps `claude`, `codex`, and `gemini` to their platform-owned origins and rejects every caller override that differs. The minter re-applies the mapping to stored rows, so legacy data cannot bypass the create-time check.
- Agent-session create/steer/rehydrate fail unavailable when `BEX_AGENT_MODEL_PROXY_URL` is absent; stored-session reads and cleanup remain available during a configuration outage. Production API/gateway manifests now configure the internal listener. `sandbox.AgentSessionLifecycle` independently rejects empty, reusable-looking, or sibling-session credentials; only the exact session placeholder may enter the Pod spec.
- The per-session Cilium admission grammar now requires the proxy-mode four-rule shape, excludes every registered vendor API from direct Pod FQDN egress, and requires exact 8082 Git + 8084 model gateway hops. Kubernetes and Cilium ingress admit 8084 only from sandbox-regime namespaces; no edge route exists.
- The proxy admits only each provider's JSON POST inference endpoints. File, batch, model-management, fine-tune, account, DELETE, and arbitrary same-origin operations fail before the vendor hop.
  - **Amended 2026-08-16 (same day):** the first cut also required an empty query string on the Anthropic and OpenAI arms. Claude Code reaches the _same_ inference operations through the Anthropic SDK's beta namespace — `POST /v1/messages?beta=true` and the `count_tokens` sibling — so the boundary refused every real Claude turn with `403 model operation is not allowed`, and each session failed within seconds of its first model call. The allowlist now admits an exact `beta=true` on those two Anthropic paths and nothing else (`allowedQuery`, which the Gemini arm's `alt=sse`/`key` handling now shares); an unknown parameter, a second value, or `beta=false` is still refused. The operation set is unchanged — only the flag that addresses it is.
- Admission happens before expensive identity/mint work where possible: 4 MiB request cap, global/per-source-Pod limiter (32/2 defaults), 2-minute read deadline, 64 MiB response cap, and 2-hour total exchange lifetime.
- The 60-second credential cache is gone. Every provider exchange re-runs the bex-api mint's current live-session check; cancellation or terminal transition prevents the next exchange immediately.

The remaining capability is intentional: malicious code in a live authorized session can consume the approved inference APIs. Provider spend caps and ADR047 phase-2 token metering complement the platform bounds.

## 4 and 9 — observability fan-out: fixed

Log `resource` arrays are rejected above 20 raw elements before deduplication, then compacted in first-seen order. The common validator is used by REST and both MCP log tools, so repeated identifiers cannot multiply authorization, Loki, or Kubernetes work.

`MetricsFilters` rejects more than eight output filters and rejects duplicates before resolving the App or querying Pods/Prometheus. Tests pin both pre-upstream rejection and log deduplication.

## 5 — authorization freshness: fixed at the shared seam

Earlier rounds added sink-adjacent fresh checks one verb at a time, which made omission the recurring failure mode. `Base.authorizeAndRecord` now routes every relation classified in `writeRelations` through `CheckFresh`; read relations retain the positive cache. Existing explicit fresh checks remain defense in depth. The regression test models cached allow + authoritative deny and proves `Authorize(can_manage)` now refuses it while `Authorize(can_view)` still uses the read cache.

## 6 — push-device cardinality: fixed

The Postgres upsert takes a transaction-scoped workspace advisory lock before the token lock, revokes a moved capability, counts active registrations, and admits at most 10 per subject and 1000 per workspace. Existing-device rotation does not consume a new slot. The lock makes concurrent final-slot attempts converge to one success; coded 409 responses distinguish subject and workspace quotas without exposing the opaque push token.

## 7 — GraphQL method/body boundary: fixed

The root mux now registers `POST /graphql`. Go's method-qualified mux returns 405 for GET/PUT/PATCH before auth or decoding, so the generic body limiter's deliberate GET exemption cannot reach GraphQL. The composed-mux inventory and an explicit method regression test pin the route shape.

## 8 — tenant shared suffix (medium): accepted external blocker

This is the seventh report of the `onbex.co` cookie-tossing risk. It is real: browser JavaScript on one sibling can set a parent-domain cookie received by another. The control plane remains on the separate registrable `bex.co` domain, so the scope is tenant-to-tenant hosting.

There is no transparent application-layer fix: stripping response `Set-Cookie` attributes does not stop `document.cookie`, and disabling the production base domain would remove the currently shipped hosting surface. The standing choices remain Public Suffix List registration after eligibility or a per-tenant registrable-domain restructure. The operator/static server continue warning loudly, and the browser isolation gate remains in `scripts/static-site-browser-isolation.mjs`.

## 12 — browser cache transition: fixed

Registration now mirrors login: it awaits `getClient().clearStore()` before invalidating the Kratos session cache and navigating. A newly registered subject cannot render entities or cached deploy-hook capabilities left by an expired prior subject in the tab-lifetime Apollo singleton.

## 13 — kubelet serving TLS (low): deferred

Unchanged fourth report. Removing `--kubelet-insecure-tls` before kubelets have CA-trusted serving certificates with valid node SANs would remove the resource-metrics service. The required work is a kubelet-serving CSR approver/certificate rollout followed by a production render assertion. Cilium WireGuard protects node transport meanwhile; this is not treated as certificate authentication.

## 14 — auth namespace reachability: fixed

The `auth` namespace now has default-deny ingress. Separate policies admit:

- auth components and migration Jobs to their own namespace;
- exact `bex-api` ports for Kratos public/admin, Hydra admin, and OpenFGA;
- the SSH gateway to OpenFGA only;
- dashboard SSR to Kratos public and Hydra admin;
- Traefik to the two public ports only;
- CNPG management and Prometheus SQL-exporter traffic only to the three auth DB clusters.

The services remain plain HTTP inside the cluster, but arbitrary platform namespaces can no longer use network location alone to reach an Ory admin port. Workload-identity mTLS remains a possible later defense for the explicitly admitted callers.

## 15 — operator admission containment: fixed

The three operator ValidatingAdmissionPolicies now cover DELETE as well as CREATE/UPDATE and validate `oldObject` for deletes. DELETE applies the namespace boundary without re-validating a legacy workload against today's authoring grammar, so cleanup remains possible. The namespace-only policy also covers PVC patch/update, kpack Builds, CNPG status, and bex CR finalizer/status subresources. A compromised controller identity can no longer use its cluster-scoped RBAC to mutate/delete those resources in platform or unrelated namespaces; canonical tenant hosting, isolated execution, and the legacy bootstrap namespace remain the only admitted targets.

## 17 — Barman Secret RBAC: fixed

The vendored v0.13.0 controller ClusterRole's broad Secret rule is removed at kustomize render time. Upstream's ObjectStore reconciler declares that marker but does not read or mutate Secrets; it creates namespaced Roles whose `resourceNames` contain only the referenced ObjectStore credentials for the CNPG instance ServiceAccount. This was verified against the official v0.13.0 source (`internal/controller/objectstore_controller.go` and `internal/cnpgi/operator/specs/role.go`). The GitOps validator fails if the controller regains a cluster-wide Secret rule. Controller and injected sidecar images are now digest-pinned as well.

## 18 — node-origin registry DNS (low): accepted constrained residual

Unchanged repeat. Arbitrary DNS registry names can make kubelet perform blind OCI/TLS probes outside tenant Pod egress enforcement. The request is protocol constrained, exposes no response body, and external registries receive no platform credential. A complete fix needs a node-level egress policy or an origin-validating registry mirror/allowlist; lexical API validation cannot safely close DNS rebinding at the later kubelet resolution point.

## 19 — privileged artifact integrity: fixed for the cited paths

- CI verifies the pinned clusterctl binary against a repository-stored SHA-256 even on cache hits.
- Cloud-init fetches k3s `install.sh` from the exact release tag, verifies its paired Terraform checksum, then executes it with an independently pinned `INSTALL_K3S_VERSION`.
- Both bex Dockerfile bases and all six native language runtime indexes are digest-pinned.
- Paketo builder/build/run inputs and both Barman controller/sidecar images are digest-pinned.
- Vendored kpack/Barman release manifests already have repository-enforced SHA-256 checks.
- The dashboard Deployment's `dashboard:latest` is only the kustomize match key; the effective production Argo Application replaces it with the checked-in `ghcr.io/bex-co/bex-dashboard@sha256:…` value, and deploy CI verifies digest write-back before rollout.

Reviewed dependency updates now change the content identity explicitly. Automated digest-bump cadence remains maintenance work, not a reason to execute a retagged artifact.

## 20 — deploy-hook token in request paths (low): accepted custody residual

Unchanged repeat of ADR072 #4. Render compatibility requires the opaque hook token in the route path. It is redacted from application logs and responses; raw Traefik access logs remain node/admin custody, and Alloy drops non-tenant platform lines before Loki. Tenants cannot read raw edge logs. Moving the credential to a header would break the copy-ready webhook contract; an edge rewrite would still observe the original request target. Rotation/revocation and restricted raw-log custody remain the controls.

## Verification

The remediation is pinned by focused Go tests for authorization freshness, provider binding, sandbox placeholder admission, model proxy operation/body/lifecycle controls, log and metrics bounds, GraphQL methods, and quota error mapping; a Postgres integration test covers the concurrent final device slot. GitOps rendering validates the model listener/API wiring, auth policies, operator admission policy objects, Barman RBAC/image contract, and kpack digests.

Executed during remediation: backend build, full tests, and `go vet`; operator `make test` (including envtest); dashboard's full 2,116-test suite and TypeScript check; changed-dashboard-file Prettier check; repository GitOps validation; all affected kustomize renders; Kubernetes 1.35 server-side dry-run/compilation of the changed ValidatingAdmissionPolicies; cloud-init YAML parse after Terraform substitution; both downloaded-artifact SHA-256 checks; and `git diff --check`.
