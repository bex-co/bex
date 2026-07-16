# Security review: RBAC, supply chain, injection surface, network isolation, secrets, OAuth

**Status:** implemented (w6/m6) · **Scope:** an evidence-backed audit of bex's security posture, not a feature. Closes `GOAL.md` #7.

This is one finding-by-finding writeup across the six audit areas of w6/m6. Each finding carries a severity, the evidence (file:line), and a remediation status: **fixed-in-w6/m6** or **follow-up-filed** (linked to a `.pm/w6/` inbox note). The audit was conducted 2026-07-11 against the tree at that date.

## Severity summary

| Area | Finding | Severity | Status |
| --- | --- | --- | --- |
| RBAC | `verbs: ['*']` wildcard on a phantom `app.bex.co services` CRD (stale scaffold) | LOW (latent) | **fixed** — scaffold deleted |
| RBAC | Unused `metrics-auth-role` (tokenreview/subjectaccessreview) + unbound `metrics-reader` | LOW | **fixed** — scaffold deleted |
| RBAC | bex-api ClusterRole grants cluster-wide `secrets`/`pods`/`pods/log` read | MEDIUM-HIGH | follow-up-filed ([w6/002](../.pm/w6/002.md)) |
| Supply chain | No image signing or SBOM anywhere in the repo | HIGH (gap) | **fixed** — cosign + SBOM in CI |
| Supply chain | CNB tenant images (in-cluster build → Zot) unsigned | MEDIUM (gap) | follow-up-filed ([w6/006](../.pm/w6/006.md)) |
| Registry authz | In-cluster Zot accepts unauthenticated push/pull/catalog from tenant build pods | HIGH (gap) | **fixed** — htpasswd + `accessControl` (anonymous denied), build-push & pull authenticated ([w7/m8](../.pm/w7/m8/README.md), [ADR022 § Registry access control](ADR022-tenant-isolation.md#registry-access-control-w7m8)) |
| Injection | SQL construction (control-plane Postgres) | — | **clean** — all parameterized |
| Injection | Webhook HMAC verification | — | **clean** — constant-time, full-body, fail-closed |
| Injection | No input validation on build `repo`/`branch`/`rootDir` | LOW (defense-in-depth) | **fixed** — validators at the API boundary |
| Network isolation | Reachability matrix declared but not live-verified | MEDIUM (gap) | **fixed** — live test + structural CI guard |
| Secrets hygiene | Plaintext credentials in logs/error paths | — | **clean** — no leaks found |
| Secrets hygiene | GitHub `APIError` echoes upstream 5xx body to callers | LOW | follow-up-filed ([w6/005](../.pm/w6/005.md)) |
| OAuth/OIDC | Token `aud` enforced, `iss` not validated | LOW-MEDIUM | **fixed** — issuer check added |
| OAuth/OIDC | PKCE not enforced bex-side (Hydra enforces for public clients) | LOW | follow-up-filed ([w6/003](../.pm/w6/003.md)) |
| OAuth/OIDC | Constant-time comparison (webhook either-key, internal CP token) | LOW | follow-up-filed ([w6/004](../.pm/w6/004.md)) |

No CRITICAL or HIGH vulnerabilities were found. The two HIGH-severity **rows** above are gaps (absent controls), now closed by CI signing and the live network test — not exploitable defects.

## 1. RBAC least-privilege

**Method.** Every `ClusterRole`/`Role` + binding in `lego/operator/config/rbac/`, `lego/operator/config/api/rbac.yaml`, `lego/operator/config/activator/`, and `deploy/gitops/` was enumerated (23 in-repo objects), each granted verb/resource cross-checked against what the component actually does. Upstream chart RBAC (Kratos/Hydra/OpenFGA/CNPG/cert-manager/Traefik/Prometheus) ships via HelmRelease and lives outside this repo — noted, not edited.

**Findings.**

- **Fixed: stale `service-{admin,editor,viewer}` roles + `metrics-{auth,reader}` roles.** The three `service-*` ClusterRoles targeted an `app.bex.co services` CRD that does not exist (the group defines `apps`/`databases`/`keyvalues` only), were unbound, and `service_admin_role.yaml` carried `verbs: ['*']` — an unjustified wildcard. The `metrics-auth-role` (+ binding) granted `tokenreviews`/`subjectaccessreviews` `create` cluster-wide for a `kube-rbac-proxy` sidecar that this repo does not run (zero `kube-rbac-proxy` references), and `metrics-reader` was unbound. All six files were deleted and removed from `lego/operator/config/rbac/kustomization.yaml`. No wildcard verb/resource grants remain in the repo.
- **Confirmed least-privilege.** The operator `manager-role` is codegen-faithful: every one of its 13 resource kinds is actually constructed in `lego/operator/internal/{controller,build,runtime}` and matches the `+kubebuilder:rbac` markers 1:1. The activator Role is tight to exactly its two `Patch` calls. The `bao-snapshot` SA has zero Kubernetes permissions (it only exchanges a projected token with OpenBao's k8s-auth). bex-api's `secrets` grant deliberately omits `delete` and `watch`. No `bind`/`escalate` verbs, no `serviceaccounts/token` creation, no `pods/exec` anywhere.
- **Outbound accounting agents (w8/m15).** The node egress meter has its own ServiceAccount and read-only `get/list/watch` on Pods and Nodes; Kubernetes cannot field-scope RBAC by node, so the process applies `spec.nodeName` and namespace selectors itself. It mounts only bpffs and its node checkpoint directory, runs with a read-only root filesystem, drops all capabilities, then adds `BPF`, `NET_ADMIN`, `PERFMON`, and `SYS_RESOURCE`: respectively load/maps, netfilter link attach, the read-only skb helper contract, and memlock compatibility. It is not privileged and has no host PID/IPC/root mount. The Postgres and Key Value SNI proxies only read their corresponding CRs, drop all capabilities, and never read credential Secrets or terminate tenant TLS. Prometheus listeners stay ClusterIP-only.
- **Follow-up-filed ([w6/002](../.pm/w6/002.md)):** bex-api (the externally-reachable service) holds a ClusterRole with cluster-wide `secrets`/`pods`/`pods/log` read — the largest blast-radius grant on the product surface. Scoping it to the served namespace(s) needs care (dynamic tenant namespaces) and is filed rather than risked mid-milestone. The operator's own cluster-wide `secrets` CRUD is justified by the multi-tenant dynamic-namespace design (ClusterRole is architecturally required); defense-in-depth follow-up is in the same note.

## 2. Supply chain — image signing + SBOM

**Gap before this milestone.** A repo-wide grep for `cosign`/`sbom`/`syft`/`sigstore` returned zero real hits. No image — neither the `lego/` product image nor CNB-built tenant images — was signed or accompanied by an SBOM.

**Fixed: platform images.** `.github/workflows/deploy.yml` now signs every pushed image keyless and attaches an SPDX SBOM, for **both** the operator and dashboard images:

- `sigstore/cosign-installer@v3` installs cosign; `anchore/sbom-action/download-syft@v0` installs syft.
- After each `docker/build-push-action` push, the workflow runs `cosign sign --yes <image>@<digest>` (keyless — the workflow's own GitHub Actions OIDC identity, recorded in the Rekor transparency log; `id-token: write` was added to the job permissions), generates `syft <image> -o spdx-json`, then `cosign attest --type spdxjson --predicate` to record the SBOM as an in-toto attestation (the `attach sbom` flow is deprecated upstream). Signing rides the content-addressed digest `build-push-action` emits, so a mutable tag can never be re-pointed under a signature.

**Verifying a pulled image.**

```bash
cosign verify \
  --certificate-identity-regexp 'https://github.com/bex-co/bex/.github' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  ghcr.io/bex-co/bex-operator@sha256:<digest>
cosign verify-attestation \
  --type spdxjson \
  --certificate-identity-regexp 'https://github.com/bex-co/bex/.github' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  ghcr.io/bex-co/bex-operator@sha256:<digest>   # the in-toto SBOM attestation
```

**Follow-up-filed ([w6/006](../.pm/w6/006.md)):** CNB tenant images built in-cluster via BuildKit and pushed to the internal Zot registry are not signed. The build Job has no OIDC identity (Zot is internal, not a public registry), and admission-time verification is explicitly out of scope for this task. The note records the deferral and the path to closing it (a signing key injected into the build Job + verification at admission).

## 3. Injection / input-validation surface

**Method.** Three surfaces audited: control-plane Postgres query construction, the git-push webhook HMAC path, and build-pipeline argument handling.

**SQL — clean.** All 62 SQL call sites in `lego/backend/internal/store/` are parameterized (`$1..$N` + args via pgx). The only `fmt.Sprintf` in any SQL-building file formats placeholder **indices** (`$%d`, `len(args)`), never user values (`audit.go:103-122`). `ORDER BY` columns/direction are static literals (no API param selects them). Cursor pagination is a parameterized keyset subquery (`audit.go:113-119`) or in-memory slicing (`core.Page`, the owners API). The sibling `lego/backend/internal/postgres/` package contains zero `Pool.Query/Exec` calls — it manages `Database` CRs via the Kubernetes API, not direct SQL.

**Webhook HMAC — clean.** The `POST /v1/webhooks/git` handler (`lego/backend/internal/apps/webhook.go`) reads the full raw body before any JSON decode, then verifies; `hmac.Equal` (constant-time, via `subtle.ConstantTimeCompare`) over SHA-256; the `sha256=` prefix is stripped before `hex.DecodeString`; only the strong `X-Hub-Signature-256` header is accepted (no sha1 fallback). Fail-closed 503 when neither `BEX_WEBHOOK_SECRET` nor `BEX_GITHUB_WEBHOOK_SECRET` is set; the 1 MiB body cap truncates oversized payloads so the recomputed MAC diverges and fails closed (a DoS guard, not a bypass).

**Build-pipeline arguments — fixed.** bex never shells out to build: repo/ref/rootDir become one BuildKit container arg (`--opt context=<repo>#<ref>:<rootDir>`, `lego/operator/internal/build/build.go`), and the operator's sole `exec.Command` (`docker inspect`) takes an operator-derived image ref. So there was no command injection **in bex's process**, but bex-api performed no validation on `repo`/`branch`/`rootDir` — delegating all robustness to BuildKit/git downstream. This milestone adds single-source validators in `lego/backend/internal/store/api.go` (`ValidRepo`, `ValidGitRef`, `ValidRootDir`), enforced at both create paths (`specFromCreate`, `appFromRequest`) and `SetRootDir`:

- **repo** must be `https://`/`http://`/`ssh://`/`git@` with no whitespace or control chars — `file://`, bare local paths, `git://`, and embedded newlines are refused (so a request can never point a build at the build pod's filesystem).
- **branch/ref** must match `^[A-Za-z0-9][A-Za-z0-9._/@+-]*$` — rejects shell metacharacters (`;`, `|`, backticks, `$()`), whitespace, control chars, and a leading dash (the git flag-injection class).
- **rootDir** must be a relative path with no `..` component, no absolute prefix, no backslash, no control chars.

Covered by `internal/store/validators_test.go` (adversarial inputs per field) and `TestSetRootDirRejectsTraversal`.

## 4. Network isolation — live verification

**Gap before this milestone.** `docs/ADR022-tenant-isolation.md` documented the threat model and a reachability matrix but had no live test backing it. (An envtest suite, `lego/operator/internal/controller/isolation_test.go`, validated the per-App NetworkPolicy is **constructed** with correct selectors — but envtest has no CNI, so it could not verify actual packet-level enforcement.)

**Fixed.** Two complementary checks now back the matrix (see `docs/ADR022-tenant-isolation.md` §Live verification):

1. **Live reachability** — `scripts/verify-tenant-isolation.sh` deploys two workspace-scoped probe pods (via App CRs that trigger the operator's per-App NetworkPolicy) and runs every allow/deny cell with real `kubectl exec` + `nc`/`curl` against a cluster with an enforcing CNI (Cilium/Calico). `make verify-tenant-isolation` (from `lego/operator/`) runs it against the current kubeconfig. Idempotent and self-cleaning. It also checks PSS-baseline rejection, SA-token automount disabled, seccomp/capabilities hardening, and namespace ResourceQuota/LimitRange presence.
2. **Structural CI guard** — `scripts/gitops-validate.sh` (CI-runnable, no cluster, runs in `.github/workflows/gitops.yml`) asserts every platform namespace carries a default-deny-ingress policy (`podSelector: {}` + `policyTypes: [Ingress]`) and that **no** allow-list peer names the tenant apps namespace (`default`). A manifest regression in `deploy/gitops/base/network-policies.yaml` now fails CI before Argo CD applies it.

The datastore rows of the matrix (workspace-A → workspace-B Postgres/Valkey) are enforced by the **same** workspace-label selector as the cross-workspace pod rows — CNPG and Valkey pods carry `app.bex.co/workspace`, so the per-App policy that blocks cross-workspace pod traffic blocks cross-workspace datastore traffic too.

## 5. Secrets hygiene — logs and error paths

**Method.** Every `log.`/`slog.`/`fmt.Errorf`/`fmt.Sprintf`/`panic(`/HTTP-error site near a secret-bearing variable in `lego/backend/` and `lego/operator/` (~49k LOC, 209 files) was inspected.

**Clean — no plaintext credential leaks found.** The highest-risk values were traced through every log/error/response site:

- **DB URI** (`BEX_CP_DB_URI`) — pgx v5.10.0 redacts passwords in all its errors (`redactPW`: `postgres://u:pw@h` → `postgres://u:xxxxx@h`); the raw URI local is held in memory and passed only to `pgxpool`/`migrate`, never to a log call.
- **OpenBao tokens** — sent only via the `X-Vault-Token` header; `secrets/store.go`'s `do()` has an explicit doc-contract that the response body is never included in errors; tenant env-var values surface only through the `RelCanViewSensitive`-gated reveal verb ("Names only in the error — never the value").
- **Webhook secrets** — used only as HMAC keys via `hmac.Equal`; error responses are static strings.
- **API keys / Hydra tokens** — sent via headers; `ClientSecret` is returned once on `Create` to the authorized caller and blanked on `List`; `core.DoJSON`'s `HTTPStatusError` excludes both bearer and response body.
- **OpenFGA key, SMTP password, GitHub App private key, install/clone tokens, k8s Secret `data`** — each is sent via header/redacted-by-dependency/name-only/written-directly-to-Secret-data, never logged. The operator never reads clone-secret values into Go memory.

No `%+v` whole-struct dump of a config/server/secret struct exists anywhere; no secret-bearing struct has a `String()`/`MarshalLogObject`.

**Follow-up-filed ([w6/005](../.pm/w6/005.md)):** GitHub `APIError.Error()` (`lego/backend/internal/github/client.go:123`) echoes the upstream response body into the error string; 5xx bodies pass through to the API caller via `core.WriteErr`. No bex credential is in that body (tokens appear only on 2xx), but it is upstream-error-text disclosure worth collapsing to a fixed message.

## 6. OAuth / OIDC correctness

**Method.** The introspection middleware (`lego/backend/internal/api/auth.go`) and RFC 9728 discovery (`server.go`) were traced against `docs/ADR012-auth.md` §7.

**Fixed: issuer (`iss`) validation.** The token **audience** check was already enforced and fail-closed when `BEX_OAUTH_RESOURCE` is set (`auth.go`: a token whose `aud` is non-empty must include the resource). But `BEX_OAUTH_ISSUER` was never compared against the introspected token's `iss` — it was only published to clients via RFC 9728 metadata. This milestone adds the matching `iss` check: when the issuer is pinned, a token whose `iss` is non-empty must match it (`auth.go introspectUpstream`). It mirrors the audience check's defensive shape — empty `iss` stays accepted, so client_credentials tokens (which need not carry `iss`) keep working. Covered by `TestIssuerValidation`.

**Confirmed correct.** RFC 9728 protected-resource metadata (`/.well-known/oauth-protected-resource`) and the 401 `WWW-Authenticate: Bearer resource_metadata=…` hint are fully implemented and gated by the same predicate (`resourceMetadataURL`), so they cannot drift (`server.go:354-362`, `auth.go:88-91,275-278`). The endpoint and hint only mount when both `OAuthIssuer` and `OAuthResource` are set and the resource parses as an absolute URL.

**Authorization boundary (matches docs).** OAuth scope is intentionally **not** consulted for authorization — authorization is delegated to OpenFGA (`core.Base.Authorize` on every verb, swept by `TestAuthzGuardsEveryVerb`). OAuth = authentication (who is the bearer?); OpenFGA = authorization (what may this subject do to this workspace object?). Any valid token (OAuth user token, client_credentials API key, Kratos session) resolves to the same `core.Identity` and hits the same per-verb gate. The only sharp edge — allow-all when `BEX_OPENFGA_URL` is unset — is the documented pre-authorization mode.

**Follow-up-filed ([w6/003](../.pm/w6/003.md)):** PKCE is not enforced bex-side. The only authorization_code clients are agent DCR clients (public), for which Hydra mandates PKCE per OAuth 2.1 conformance — so the actual MCP-agent surface is covered — but nothing in bex or the committed Hydra values requires PKCE for **all** authorization_code clients. The note records the Hydra-side `oauth2.pkce` knob that would close it.

## Follow-up register

All seven findings filed during this review were **implemented 2026-07-12** as the w6 follow-up batch (notes moved to `.pm/w6/done/`):

- [w6/002](../.pm/w6/done/002.md) — ✅ bex-api `secrets`/`pods`/`pods-log`/`metrics` moved from the cluster-wide ClusterRole to a `default`-namespace-scoped Role+RoleBinding (`deploy/gitops/base/bex-api-apps-rbac.yaml`); the ClusterRole now holds only the `app.bex.co` verbs.
- [w6/003](../.pm/w6/done/003.md) — ✅ PKCE required at the dashboard consent acceptor (`hydra-consent.ts`); a consent request whose authorize URL lacks a `code_challenge` is rejected. (Hydra has no global toggle.)
- [w6/004](../.pm/w6/done/004.md) — ✅ webhook `verify` computes every configured key's MAC with no early return; the internal tenant-API bearer uses `crypto/subtle.ConstantTimeCompare`.
- [w6/005](../.pm/w6/done/005.md) — ✅ `APIError.Error()` returns status only; the upstream response body is no longer interpolated into error responses.
- [w6/006](../.pm/w6/done/006.md) — ✅ opt-in tenant-image signing in the build Job (`build.go`: build+push as an initContainer, cosign as the main container), gated behind `BEX_TENANT_SIGNING_KEY_SECRET`. Admission-time verification closed by w7/m11: a `ValidatingWebhookConfiguration` backed by `internal/webhook.PodAdmitter` rejects pods with unsigned or tampered tenant images when `cosign.pub` is present in the Secret.
- [w6/007](../.pm/w6/done/007.md) — ✅ `+kubebuilder:validation:Pattern`/`:MaxLength` markers on `Repo`/`Branch`/`RootDir` (`lego/types`); CRD regenerated so hand-applied App CRs can't bypass the input validators.
- [w6/008](../.pm/w6/done/008.md) — ✅ two pre-existing red `internal/apps` autoscaling tests fixed (root cause: missing `Context` in `graphql.Params` — graphql-go v0.8.1 doesn't default a nil context).

**Closed by w7/m35 (2026-07-15) — ✅.** Outbound webhook SSRF gap: `lego/types/netutil` now exports `SafeDialContext` (shared guard: loopback/private/link-local/unspecified/cloud-metadata blocked at dial time, DNS-rebind covered by checking all resolved IPs, no redirect-following via `http.ErrUseLastResponse`). `worker.go`'s `defaultClient` uses it; the activator's maintenance-page fetch uses the same shared implementation. Test battery: `TestUnsafeOriginIP`, `TestSafeDialContextBlocksPrivateAddresses`, `TestSafeDialContextDNSRebindBlocked` in `lego/types/netutil/ssrf_test.go`; `TestDefaultClientSSRFGuard` in `worker_test.go`.

**Found 2026-07-15 (w1/m37, discovered while live-verifying on the mock cluster), fixed same day (w10/m1, `fa49fbbd`) — ✅.** The manager's controller-runtime cache never finished syncing on a clean start, so no controller (App/Database/KeyValue) ever began reconciling — confirmed live: after `make deploy` + a pod restart, the log showed every `Starting EventSource` line (including `KeyValue`'s `*v1.Secret` source) but never reached `Starting Controller`/`Starting workers`, while `controller-runtime.cache.UnhandledError` repeated forever: `failed to list *v1.Secret: secrets is forbidden ... at the cluster scope`. Root cause: the KeyValue controller's `Owns(&corev1.Secret{})` registered a cluster-wide Secret watch, but `manager-role`'s ClusterRole deliberately excludes `secrets` (the w6/002 hardening above) in favor of a namespace-scoped `Role` (`deploy/gitops/base/operator-apps-rbac.yaml`) that a cluster-wide watch could never satisfy, since the manager's cache had no `ByObject` restriction scoping the watch to match. Not new damage from m37 — the gap existed since the KeyValue controller shipped (ADR021) — m37 was just the first time anyone restarted a manager against today's RBAC + code in a way that surfaced it. Fixed by `internal/controller/secret_cache.go`'s `NamespacedSecretCacheOptions`, wired into the manager's `Cache` option in `cmd/manager/main.go`, restricting the Secret watch to the apps namespace so the existing namespace-scoped Role becomes sufficient — verified live by `secret_cache_test.go` ("starts App and KeyValue watches without cluster-wide Secret permission").

## Out of scope

- Dependabot triage (36 findings) is owned by `w1/m23/t002` — excluded here.
- Namespace-per-workspace isolation is deferred (`docs/ADR022-tenant-isolation.md` §Rejected options).
- Admission-time tenant-image signature verification: **closed by w7/m11** (2026-07-13). A `ValidatingWebhookConfiguration` backed by `internal/webhook.PodAdmitter` enforces cosign signatures on tenant images when `BEX_TENANT_SIGNING_KEY_SECRET` is set with `cosign.pub`. Off by default; byte-identical when unset.
