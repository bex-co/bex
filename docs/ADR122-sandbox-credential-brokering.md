# ADR122 — Sandbox Group Credential Brokering

**Status:** accepted (2026-11-22, production-verified on pin `de0ed1cbfe7f`; open divergences awaiting user sign-off recorded below) · **Extends:** [ADR008 § Render parity first](ADR008-vision.md#render-parity-first), [ADR013](ADR013-secrets.md), [ADR018](ADR018-render-parity.md), [ADR042](ADR042-sandbox-cluster-substrate.md), [ADR062](ADR062-sandbox-credential-vault.md), [ADR104](ADR104-sandbox-render-parity.md) · **Amends:** [ADR104 § Out of scope](ADR104-sandbox-render-parity.md#out-of-scope), [ADR018 § Render operation omissions](ADR018-render-parity.md#render-operation-omissions) · **Board:** `w2/m532` (control plane, store, OpenBao custody, 7 REST routes), `w2/m533` (GraphQL, MCP, Dashboard), `w2/m534` (Pod-bound HTTPS broker on `ssh-gateway`), `w2/m535` (Pod wiring, Cilium policy, broker CA trust), `w2/m536` (security review [ADR123](ADR123-security-review-sandbox-credentials.md), CLI/SDK compat, production check — done 2026-11-22)

## Context

Render added seven sandbox-group credential operations to its Public OpenAPI specification in round-7 drift (2026-11-13, `w3/m367`) and exposed `render sandbox-groups credentials {list,create,get,update,delete,versions,restore}` in `render-oss/cli` v2.36:

- `GET /v1/sandbox-groups/{sandboxGroupId}/credentials` (`list-sandbox-group-credentials`)
- `POST /v1/sandbox-groups/{sandboxGroupId}/credentials` (`create-sandbox-group-credential`)
- `GET /v1/sandbox-groups/{sandboxGroupId}/credentials/{credentialId}` (`retrieve-sandbox-group-credential`)
- `PATCH /v1/sandbox-groups/{sandboxGroupId}/credentials/{credentialId}` (`update-sandbox-group-credential`)
- `DELETE /v1/sandbox-groups/{sandboxGroupId}/credentials/{credentialId}` (`delete-sandbox-group-credential`)
- `GET /v1/sandbox-groups/{sandboxGroupId}/credentials/{credentialId}/versions` (`list-sandbox-group-credential-versions`)
- `POST /v1/sandbox-groups/{sandboxGroupId}/credentials/{credentialId}/restore` (`restore-sandbox-group-credential`)

Previously, [ADR104 § Out of scope](ADR104-sandbox-render-parity.md#out-of-scope) deferred credential brokering because `render.com/sandboxes` marked it early access, and `lego/backend/internal/sandbox/notoffered.go` answered all seven routes with `404 SANDBOX_CREDENTIALS_UNSUPPORTED`. Under [ADR008 § Render parity first](ADR008-vision.md#render-parity-first), closing these seven operations closes the last non-excluded omitted operation family in Render's 264-operation Public API.

Sandboxes already ship groups (`w22/m311`, `w2/m359`), images (`w2/m358`), snapshots (`w32/m285`), network policies (`w32/m283`), and scoped API key permission mappings (`w9/m496`: `KeySandboxesHistory` and `KeySandboxesCredentials` already map all seven REST routes in `lego/backend/internal/api/key_permission_ops.go`).

## Decision

> **Note on attribution:** Decisions D2, D5, and D6 below carry architectural recommendations from the 2026-11-18 `/pm-brainstorm` resting on [ADR008 § Render parity first](ADR008-vision.md#render-parity-first) and existing bex platform architecture (ADR013, ADR062, ADR098, ADR112). They are **not** explicit user sign-offs; open divergences awaiting user sign-off are explicitly flagged in [§ Open divergences awaiting user sign-off](#open-divergences-awaiting-user-sign-off).

### D1 — Control-plane data model and pinned Render limits

All seven operations match the pinned Render OpenAPI contract (`lego/backend/internal/api/openapi/render-public-api-1.json`) field for field and status code for status code:

| Rule | Contract |
| --- | --- |
| Credential ID (`id`) | Case-sensitive caller-assigned string matching `^[A-Za-z0-9._~-]+$`, minLength 1, unique within `sandbox_group_id`. `.`, `..`, and any ID starting with `rnd-` are reserved and refused (`400`). Immutable after creation. |
| Destination hostname (`domain`) | Public DNS hostname, normalized to lowercase and matched exactly (no wildcard matching). Rejects IP literals (IPv4/IPv6), ports, schemes, paths, wildcards (`*`), single-label hostnames, and local/private/cluster hostnames (reusing `sessionegress.PrivateOrClusterHost`). |
| Secret value (`value`) | Valid UTF-8 string up to **5 KiB (5,120 bytes)**. Write-only: never returned by any endpoint (including to Admins), never stored in Postgres, never written to audit events or logs. |
| Enabled (`enabled`) | Boolean, defaults to `true` on create. |
| HTTP methods (`methods`) | Subset of `GET`, `HEAD`, `POST`, `PUT`, `PATCH`, `DELETE`, `OPTIONS`, normalized uppercase and deduplicated. Omitted or `[]` matches any method. Always serialized as `[]` (never `null`). |
| Path patterns (`paths`) | Absolute path patterns starting with `/`. `*` matches one nonempty path segment; `**` matches one or more segments. Wildcards must occupy whole segments (e.g., `/a/*` matches `/a/b`, `/a/**` matches `/a/b/c`; neither matches `/a`). Patterns forbid query strings (`?`), fragments (`#`), and escaping (`\`). Omitted or `[]` matches any path. Always serialized as `[]` (never `null`). |
| Injection (`injection`) | Object with required `type` in `header \| basic \| cookie \| query`. `name` is required for `header`, `cookie`, and `query` and **forbidden** for `basic`. `header` rejects routing/connection headers (`Host`, `Connection`, `Keep-Alive`, `Proxy-Authenticate`, `Proxy-Authorization`, `TE`, `Trailer`, `Transfer-Encoding`, `Upgrade`, `Content-Length`, `X-Forwarded-*`, `Forwarded`, and control/non-token header names). `template` is optional (default `${SECRET}` when omitted; omitted on read when default/unset or echoed when set); when provided, must contain **1 to 5** occurrences of `${SECRET}`. For `basic`, the rendered value (applying `template` to `value`) must have the form `username:password` (nonempty username, colon, no CR/LF). |
| Per-group caps | Maximum **25 credentials** per sandbox group. Maximum **200 KiB (204,800 bytes)** combined serialized size of all current credential settings and secret values in the group. Maximum **1 MiB (1,048,576 bytes)** total storage across the group including retained version metadata and retained secret values. Exceeding a cap refuses the write (`400`). |
| PATCH semantics | Omitted top-level fields preserve current values. Providing `injection` replaces the entire `injection` object (`template` defaults to `${SECRET}` when omitted). Empty `methods: []` or `paths: []` clears that filter. Setting `value` creates a new version even if the secret value is unchanged. Changes to routing, injection, or `enabled` create a new version. |
| Versioning & retention | Each credential has a monotonic `int64` version ID starting at `1` (`version.id >= 1`), incremented on every create, update, or restore. `restoredFromVersionId` is set only on versions created by `restore` and omitted on create/update versions. Each credential retains its **current configuration** plus the **3 most recently active distinct prior configurations** (identified by fingerprint). When a 4th distinct prior configuration is evicted, **all** version rows referencing that evicted configuration are deleted, and its OpenBao secret entry is purged. |
| List & status codes | `GET .../credentials` returns a bare JSON array sorted lexicographically by `id` (`[]` when empty). `GET .../versions` returns a bare JSON array sorted newest version ID first. `POST .../credentials` answers `201 Created`; `GET`/`PATCH`/`POST .../restore` answer `200 OK`; `DELETE` answers **`202 Accepted`** with an empty body and is idempotent (repeated deletes or absent credentials in an existing group answer `202`). Temporary write contention on the group lock answers `429` with `Retry-After` and `RateLimit-*` headers (`429SandboxCredentialRateLimitOrConflict`). |

### D2 — Group-salted SHA-256 fingerprint formula

Each sandbox group carries a random 32-byte salt stored in Postgres (`sandbox_groups.credential_fingerprint_salt`, lazily generated inside the group's first credential write transaction under the group write lock and never returned by any API).

For a configuration `(domain, enabled, methods, paths, injection.type, injection.name, injection.template, value)`:

1. Normalize `domain` (lowercase), `methods` (uppercase, deduplicated, sorted), `paths` (deduplicated, preserving canonical order or sorted consistently), `injection.type`, `injection.name` (trimmed/canonical), `injection.template` (empty string normalized to `"${SECRET}"` so omitting `template` and passing `"${SECRET}"` hash identically).
2. Compute `HMAC-SHA256(key = group_salt, message = canonical_payload)` where `canonical_payload` is a length-prefixed / unambiguous canonical JSON encoding of the normalized configuration fields plus the raw UTF-8 `value`.
3. Hex-encode the 32-byte digest as 64 lowercase hexadecimal characters (`^[0-9a-f]{64}$`).

Properties guaranteed:

- Matching configurations (same routing, injection, enabled state, and secret value) within the same sandbox group produce the exact same fingerprint, including across a restore.
- Fingerprints cannot be compared across different sandbox groups because each group has an independent 256-bit random salt.
- Knowledge of a fingerprint does not allow offline brute-forcing of low-entropy secrets without the group salt.

### D3 — OpenBao write-only secret custody and disposal invariants

_Recommendation resting on ADR013, ADR098, and ADR112 (not a user decision)._

Raw secret values are **never** stored in Postgres. Postgres stores only metadata (`sandbox_group_credentials`, `sandbox_group_credential_versions`), per-config serialized byte counts (so the 200 KiB and 1 MiB caps are enforced atomically in Postgres without reading secrets back from OpenBao), and an opaque `secret_ref` (the 64-hex configuration fingerprint).

OpenBao KV v2 path layout (using the shared `core.SecretKV` under the `tenants/` mount via `secrets.WithTenant(ctx, workspaceID)`):

```text
tenants/data/<workspaceID>/sandbox-groups/<sandboxGroupID>/credentials/<sha256(credentialID)>/<secretRef>
```

Each KV entry stores `{"value": "<raw secret>"}`. Hashing `credentialID` to 64 lowercase hex prevents path traversal, case-folding collisions, or special-character issues with `.` / `~` / `_` while staying deterministic.

Write and disposal invariants:

1. **Fail closed when OpenBao is unconfigured or unavailable:** credential writes and internal broker reads return a sanitized `503` (`SANDBOX_CREDENTIALS_UNAVAILABLE`) and never fall back to Postgres plaintext or leak secret material or OpenBao paths in errors or logs.
2. **Write-then-commit with rollback cleanup:** on create or update with a new `secretRef`, `bex-api` writes the OpenBao entry before committing the Postgres transaction. If the Postgres transaction fails or aborts, `bex-api` best-effort deletes the newly written OpenBao entry (unless a pre-existing committed version already referenced that exact `secretRef`).
3. **Version-pruning purge:** when an update or restore evicts one or more prior distinct configurations (past current + 3 prior distinct configs), Postgres returns the evicted `secret_ref`s and `bex-api` deletes those OpenBao paths post-commit.
4. **Credential delete purge:** `DELETE` removes the credential row (cascading all version rows) and deletes all of that credential's OpenBao entries (both DB-returned `secret_ref`s and a KV prefix list under `<sha256(credentialID)>`) idempotently.
5. **Sandbox-group delete purge:** `sandbox.Service.finishGroupDelete` (`lego/backend/internal/sandbox/groups.go`) purges all OpenBao credential secrets for the group before deleting the `sandbox_groups` row; failures keep the group in `deleting_at` so the group-delete sweeper retries until OpenBao is clean.
6. **Workspace teardown purge:** `sandbox.WorkspacePurger.PurgeWorkspace` (`lego/backend/internal/sandbox/purger.go`, pre-cascade) purges all OpenBao sandbox-group credential secrets for the workspace before the `tenants` cascade removes `sandbox_groups`.

### D4 — Split RBAC, scoped API keys, and audit events

Authorization follows Render's exact Admin vs Developer split on a single `s.Authorize` gate per verb (preserving `TestAuthzGuardsEveryVerb`, `TestAuditCoversEveryWriteVerbExactlyOnce`, and `TestRoleLadderDenyMatrix`):

| Operation | Workspace relation (`model.fga`) | Required scoped API key action | OAuth scope class |
| --- | --- | --- | --- |
| `ListGroupCredentials`, `GetGroupCredential`, `ListGroupCredentialVersions` | `can_view` (Viewer+) | `KeySandboxesHistory` (`sandboxes` Read) | `bex.read` |
| `UpdateGroupCredential` touching **only** `injection`, `methods`, `paths`, `enabled` | `can_create` (Developer+) | `KeySandboxesCredentials` (`sandbox_credentials` Write) | `bex.write` |
| `UpdateGroupCredential` touching `domain` or `value` | `can_manage` (Admin only) | `KeySandboxesCredentials` (with Admin creator ceiling) | `bex.write` |
| `CreateGroupCredential`, `DeleteGroupCredential`, `RestoreGroupCredential` | `can_manage` (Admin only) | `KeySandboxesCredentials` (with Admin creator ceiling) | `bex.write` |

Additional security boundaries:

- **Cross-workspace isolation:** every verb resolves the group's workspace via `scopeGroup` (`ScopeByVisibleID`), so a foreign workspace's `sandboxGroupId` answers `404 SANDBOX_GROUP_NOT_FOUND` rather than leaking existence.
- **Audit trail:** every write verb (`CreateGroupCredential`, `UpdateGroupCredential`, `DeleteGroupCredential`, `RestoreGroupCredential`) records a workspace audit event via `core.Base.Authorize` (and every denial records an audit denial). Secret values never enter audit events.
- **Account deletion (`updated_by`):** `sandbox_group_credential_versions.updated_by` stores the raw caller subject so `CleanupAccountSubject` anonymizes deleted accounts; API responses project `updated_by` through `core.PublicUserIDs` (`usr-…` for humans).

### D5 — Deterministic overlap tie-breaking

_Recommendation resting on Render's OpenAPI documentation (not a user decision)._

Render's OpenAPI specification states: _"Avoid overlapping credential configurations for the same hostname. Multiple matching credentials are not combined."_ It does not publish a tie-breaking order when multiple enabled credentials match the same request.

bex resolves overlapping matches deterministically by **lexicographical ascending `credentialId`** (the same order `GET .../credentials` returns). The first enabled credential matching `(domain, method, path)` is injected; no second credential is applied to the same request.

### D6 — Pod-bound outbound HTTPS broker data plane (`w2/m534`–`m535`)

_Recommendation resting on existing `ssh-gateway` proxy architecture (ADR062, `egressproxy`, `modelproxy`; not a user decision)._

Runtime credential injection (implemented in `w2/m534` and wired to sandbox Pods in `w2/m535`) uses a dedicated Pod-bound outbound HTTPS proxy listener on `ssh-gateway` rather than transparent host-level TLS interception:

1. **Pod identity binding:** `ssh-gateway` authenticates every inbound proxy connection by resolving the source Pod IP via the Kubernetes Pod informer to `(namespace, podName, sandboxID, workspaceID, sandboxGroupID)` — never trusting caller-supplied headers or tokens.
2. **Selective MITM:** for `CONNECT host:443`, `ssh-gateway` queries `bex-api`'s internal broker endpoint (HMAC-authenticated on `:8091`) for the group's enabled credentials matching `host`:
   - **No enabled credential for `host`:** pure TCP pass-through TLS tunnel (no TLS termination, no certificate substitution).
   - **At least one enabled credential for `host`:** terminates TLS with a short-lived leaf certificate signed by the cluster's Sandbox Broker CA (mounted into sandbox Pods via `SSL_CERT_FILE` / system trust bundle in `w2/m535`), matches `(method, path)`, injects the winning credential (header, Basic auth, cookie, or query parameter), and forwards to the verified public upstream over TLS 443.
3. **Network-policy intersection & SSRF floor:** the broker enforces the sandbox's `networkPolicy` (`deny-all` / `allow-list` / `allow-all`) and refuses private, loopback, link-local, metadata, and cluster IP ranges at dial time after DNS resolution.
4. **No redirect following:** the broker never follows upstream HTTP redirects automatically, preventing credential leakage to a redirected host.
5. **Egress confinement:** when a sandbox group has credentials (or on credential-capable sandboxes), Cilium egress rules route outbound 443 through the broker while keeping the ADR042/ADR043 platform isolation floor intact.

## Open divergences awaiting user sign-off

1. **Proxy-based interception (`HTTPS_PROXY` + Sandbox Broker CA) vs Render's Firecracker transparent TCP-443 interception (`w2/m534`–`m535`):** clients inside a bex sandbox that ignore standard `HTTPS_PROXY`/`HTTP_PROXY` environment variables or pin specific upstream TLS root certificates will not receive injected credentials. Recorded in [ADR018](ADR018-render-parity.md) as a divergence awaiting user sign-off; closing it without proxy env vars would require transparent TPROXY/eBPF interception on the gVisor node pool.
2. **Overlap tie-breaking order (D5):** Render documents only that multiple matching credentials are not combined and does not specify precedence. bex selects the lexicographically smallest matching `credentialId`. If Render's live precedence is later measured and differs, bex will align to it.
3. **Write-only OpenBao secret storage (D3):** secret values live in OpenBao under `tenants/<ws>/sandbox-groups/<sbg>/credentials/<sha256(id)>/<fingerprint>` rather than Postgres; this is an internal custody choice with no customer-visible API divergence, recorded here for transparency.

## Consequences

- `lego/backend/internal/sandbox/notoffered.go` is retired and all seven `*-sandbox-group-credential*` entries leave `renderOperationOmissions` in `w2/m532`.
- `w2/m533` adds GraphQL, MCP, and Dashboard parity on top of the shared `sandbox.Service` verbs.
- `w2/m534`–`m535` add runtime injection on `ssh-gateway` and Pod/Cilium wiring.
- `w2/m536` runs the focused security review, CLI/SDK compatibility verification, and live production acceptance walk.
