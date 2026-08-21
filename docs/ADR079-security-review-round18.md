# ADR079: Security review round 18 — oEDLeI disposition

- **Status**: Accepted (2026-08-21)
- **Scan**: codex-security `oEDLeI`, repository revision `97ba1dc0` (reviewed against HEAD after ADR077), 10 findings (1 critical, 2 high, 4 medium, 3 low)
- **Lineage**: eighteenth pass in the ADR028 → … → ADR076 → ADR077 lineage

## Summary

Eight findings are fixed in place with regression tests. Finding 1 (critical, plaintext `.env` credential custody) is partially fixed in the repository — permission hardening shipped — with credential rotation/rekey recorded as the required out-of-band operator action. Finding 9 is the standing onbex.co PSL residual (thirteenth report). Round-18 #10 also closed a piece of the ADR060 §D7 digest-pinning inventory (the gVisor node bootstrap), and in doing so found the existing bootstrap URL scheme was already broken upstream.

| # | Finding | Severity | Disposition |
| --- | --- | --- | --- |
| 1 | Plaintext root credentials co-located in worktree `.env` | critical | **Partially fixed** — `.env` chmod 0600 + `set_env_var` re-pins 0600 on every write; rotation/rekey is the remaining operator action |
| 2 | GraphQL writable SQL drops the sensitive capability | high | **Fixed** — writable `ExecuteQuery` requires can_create **and** can_view_sensitive, admission + fresh sink |
| 3 | Co-located App can mount the static publisher Secret | high | **Fixed** — `StaticStore.Secret` joins the operational-name denylist; provisioned Secret stamped `protected-from-tenant-mount` |
| 4 | Webhook exact-URL read bypasses OAuth scope | medium | **Fixed** — `mayManageWorkspace` composes `RequireCapability(can_manage)` |
| 5 | Usage billing projection bypasses OAuth scope | medium | **Fixed** — `mayManageBilling` composes `RequireCapability(can_manage_billing)` |
| 6 | Static-server memory ceilings jointly exhaust the replica | medium | **Fixed** — budgets halved to 1 GiB aggregate of the 2 GiB limit + `GOMEMLIMIT` + capacity-charged cache |
| 7 | Unbounded custom domains | medium | **Fixed** — per-service/per-workspace quotas + CRD `MaxItems` + bounded TLS history |
| 8 | Sandbox owner override bypasses OAuth scope on reads | low | **Fixed** — `isWorkspaceAdmin` composes `RequireCapability(can_manage)` |
| 9 | Shared onbex.co tenant suffix | low | **Accepted residual** — onbex.co PSL (thirteenth report); `.pm/DO_NOT_DO.md` `#PSL` |
| 10 | runsc authenticated only with same-origin checksums | low | **Fixed** — repo-pinned SHA-512 digests, fail-closed arch gate; also repaired the already-broken release URL scheme |

## Finding 1 (critical) — `.env` credential custody (partially fixed)

The ignored, then-mode-0644 root `.env` aggregates the Hetzner, Terraform-state, bex-api, registry, OpenBao root-token, and a complete three-share unseal quorum; repository scripts consume it at privileged sinks. The single-file custody model itself is the documented single-operator design ([ADR013](ADR013-secrets.md), [ADR019](ADR019-infra-credentials.md)); the scan's valid new observation is that nothing enforced restrictive permissions on the file.

**Fixed in place.** The local `.env` is now mode 0600, and `set_env_var` in `scripts/bao-init.sh` re-pins 0600 on **every** write — the awk/mv replace path otherwise recreates the file at the umask default (0644). `scripts/bao-init.test.sh` asserts both the append and replace paths leave 0600.

**Remaining operator action (out of band, not automatable from the repo):** revoke and rotate every credential the file held while world-readable (Hetzner, Wasabi/state, registry, bex-api), and rekey OpenBao per [ADR037](ADR037-openbao-rekey-runbook.md) with the root token treated as compromised. Longer term the Shamir quorum should move to separate custodians or an auto-unseal system; that is a custody decision, not a code change.

## Finding 2 (high) — writable SQL capability union

`ExecuteQuery(allowWrites=true)` substituted can_create for can_view_sensitive at both the admission authorize and the round-9 fresh sink check, and the GraphQL dispatcher classifies the mutation as write-only — so a bex.write-only third-party token could read arbitrary rows through a writable SELECT. REST (`OpClassSensitive`) and MCP (read-only `Query`) did not share the gap, but the service enforced only one relation on every surface.

**Fix.** `lego/backend/internal/postgres/query.go`: writable mode now requires **both** relations — `AuthorizeDatabase(can_create)` admission (single fetch) plus `AuthorizeDatabaseFresh(can_view_sensitive)` on the resolved Database, and `executeAuthorizedQuery` re-checks both uncached at the sink. Because `core.Base.checkAuthz` composes `checkCapability` per relation, a scoped token needs bex.write **and** bex.sensitive; scope-exempt identities (machine keys, browser sessions) are unchanged. The fresh seam is audit-silent, so no spurious allowed-read audit rows. `TestExecuteQueryWriteRequiresBothCapabilities` pins write-only-denied, sensitive-only-denied, both-allowed, and read-only-unchanged. Dispatcher classifications were deliberately not changed — the union lives at the service layer so no write-classed dispatch can skip the sensitive half.

## Finding 3 (high) — static publisher Secret mount (co-located mode)

Two parallel controls both missed the static publisher credential: the operational-name denylist in `rejectProtectedSecretRefs` omitted `r.StaticStore.Secret`, and `scripts/static-s3-credentials.sh` created the Secret without the round-12 `app.bex.co/protected-from-tenant-mount` label. Production is not reachable (`BEX_BUILD_NAMESPACE: bex-build`, `lego/operator/config/manager/manager.yaml`); the vulnerable shape is the supported co-located dev mode.

**Fix.** `StaticStore.Secret` joins the unconditional name denylist (all six projection fields, empty-guarded like the others); the provisioning script stamps the protected label byte-identical to the shared `types` constant; `scripts/gitops-validate.sh` grep-guards the label so it cannot regress. `TestRejectConfiguredOperationalSecretNames` sweeps the credential through every projection field. Operational note: clusters whose Secrets were applied by an older script revision need one re-run of `static-s3-credentials.sh` to pick up the label; the name denylist covers the gap.

## Findings 4 / 5 / 8 (medium / medium / low) — capability-blind audit-silent helpers

One root pattern, three sites: an audit-silent response-shaping helper called the raw OpenFGA checker directly, bypassing the `core.Base` seam that composes the relation's mapped OAuth capability — so a **bex.read-only** third-party token delegated by a privileged human saw data the relation's bex.write capability should gate: exact credential-bearing webhook destination URLs (`mayManageWorkspace`), the real Stripe cost/invoice/credit projection (`mayManageBilling`), and cross-owner sandbox records (`isWorkspaceAdmin`).

**Fix.** Each helper now calls `id.RequireCapability(relation)` before the fresh check and treats failure as not-privileged (`false` / `false, nil` → redact or owner-only fallback), staying audit-silent — the per-viewer denial noise is why the raw-checker seam exists. Capability-exempt identities are byte-identical; every `isWorkspaceAdmin` call site (list/get/terminate/exec-ticket) was verified to fail closed to non-admin, and the exec/lifecycle paths are independently gated write-class at the dispatcher regardless. Regression tests per package assert read-only-token redaction and full-capability reveal across the REST/GraphQL/MCP-shared service.

## Finding 6 (medium) — static-server aggregate memory budget

The independent ceilings (256 MiB cache + 512 MiB live-body retention + 32×32 MiB fetch reservations = 1.75 GiB) summed to 87.5% of the 2 GiB cgroup limit before allocator/runtime/transient headroom.

**Fix.** Live-body lease 512→256 MiB and fetch gate 32→16 (aggregate now 1024 MiB = 50% of the limit, documented next to the constants); `GOMEMLIMIT: 1500MiB` on the Deployment so GC ramps before the OOM killer; the object cache now charges `cap(body)` instead of `len`. `TestAggregateBudgetInvariant` fails any future knob bump that breaks the 50% invariant. No GitOps overlay overrides the limit.

## Finding 7 (medium) — custom-domain cardinality quotas

Verified hosts fanned out unboundedly into Ingress rules, cert-manager entries, host caches, and a permanent TLS-history annotation.

**Fix.** `BEX_MAX_CUSTOM_DOMAINS_PER_SERVICE` (default 100 — the round-12 routes/headers precedent) and `BEX_MAX_CUSTOM_DOMAINS_PER_WORKSPACE` (default 500; `0` disables either), enforced count-then-write in the claim path before DNS verification with coded `CUSTOM_DOMAIN_LIMIT` (409) identical across surfaces; the create/blueprint funnel rejects an over-cap host set with 400 before any claim. CRD schema: `MaxItems=100` on `hosts`, `MaxProperties=100` on `hostRedirects` (regenerated, envtest-proven: 101 rejected, 100 accepted). TLS history capped at 500 entries, trimming only no-longer-in-spec names (live names never dropped; deletion never depended on the annotation). Residual: the quota is the same service-level count-then-write shape as `ENV_GROUP_LIMIT`/`GIT_CONNECTION_LIMIT` — concurrent claims can overcommit by the concurrency factor; strict atomicity would move the count inside `AddDomainClaim`'s transaction and is deferred.

## Finding 9 (low) — onbex.co PSL (accepted, thirteenth report)

Unchanged standing residual: ADR055 F9 → ADR072 #1 → ADR061 #4 → ADR063 #3 → ADR064 #8 → ADR069 #3 → ADR073 #6 → ADR076 #10 → ADR077 #8 → here. `.pm/DO_NOT_DO.md` `#PSL` holds the decision; re-open at the open-signup gate.

## Finding 10 (low) — repo-pinned gVisor digests

The sandbox node bootstrap authenticated runsc/containerd-shim-runsc-v1 with checksums fetched from the same origin as the binaries; a compromised release bucket could replace both, and runsc executes with node runtime authority over the hostile sandbox pool.

**Fix.** `infra/clusterapi/overlays/hetzner-caph/sandbox-pool.yaml` embeds the reviewed upstream-published SHA-512 digests for gVisor release `20260622.0`/`x86_64` (verified against the live objects twice on 2026-08-20; no signed provenance exists for this release), fails closed on any non-x86_64 arch, and no longer downloads any checksum file. The fix also repaired a pre-existing breakage: the old URL scheme (`release-<version>` + `.sha256`) 404s upstream today. `scripts/clusterapi-validate.sh` now guards the overlay (no co-fetched checksum, arch gate, `sha512sum -c`, pinned-digest count, versioned-never-`latest` URL) and its fixture suite includes a runtime tamper proof: a tampered binary **with a matching tampered same-origin checksum** is rejected by the pinned verification while the retired comparison accepts it.
