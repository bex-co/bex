# w2 · m532 — Sandbox group credential store, OpenBao custody, versioning & all 7 REST routes

**Worker:** worker2 **Goal:** Serve all 7 pinned Render OpenAPI operations for sandbox group credentials (`/v1/sandbox-groups/{sandboxGroupId}/credentials*`) with write-only OpenBao secret custody, monotonic versioning (current + 3 prior distinct configurations with group-salted SHA-256 fingerprints), split Admin/Developer RBAC, scoped-API-key enforcement, and full OpenAPI conformance. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Write the sandbox credential brokering ADR (amending ADR104 § Out of scope and ADR018 § Render operation omissions) — **DONE** | 45m | — |
| t002 | Add Postgres schema & store for `sandbox_group_credentials` and retained version history — **DONE** | 1h | t001 |
| t003 | Wire OpenBao write-only secret custody and disposal across credential, group, and workspace lifecycles — **DONE** | 1h | t002 |
| t004 | Implement Render validation & core `sandbox.Service` credential verbs with split RBAC and audit events — **DONE** | 1h15m | t003 |
| t005 | Mount all 7 `/v1/sandbox-groups/{sandboxGroupId}/credentials*` REST routes and retire `NotOfferedRoutes` + `renderOperationOmissions` entries — **DONE** | 1h | t004 |
| t006 | Add real-DB + OpenBao + OpenFGA integration tests and `TestRenderConformance` coverage across all 7 operations — **DONE** | 45m | t005 |
| t007 | Render parity — **DONE** | 20m | t006 |
| t008 | Simplify — **DONE** | 15m | t007 |
| t009 | Test coverage — **DONE** | 30m | t007 |
| t010 | Closeout — **DONE** | 10m | t008, t009 |

## Definition of done

- All 7 pinned Render OpenAPI operations (`list-sandbox-group-credentials`, `create-sandbox-group-credential`, `retrieve-sandbox-group-credential`, `update-sandbox-group-credential`, `delete-sandbox-group-credential`, `list-sandbox-group-credential-versions`, `restore-sandbox-group-credential`) are mounted on `/v1/sandbox-groups/{sandboxGroupId}/credentials*`, pass `TestRenderConformance` and `TestEveryRenderOperationMountedOrDocumented` with all 7 sandbox-credential entries removed from `renderOperationOmissions`, and `lego/backend/internal/sandbox/notoffered.go` is retired.
- Validation enforces every pinned Render constraint: case-sensitive `id` (`^[A-Za-z0-9._~-]+$`, rejecting `.`, `..`, `rnd-*`), lowercase exact public DNS `domain` (rejecting IPs, wildcards, ports, cluster/private names), 5 KiB `value`, 25 credentials/group, 200 KiB current combined serialized size, 1 MiB total versioned size, `methods` normalization (`GET`/`HEAD`/`POST`/`PUT`/`PATCH`/`DELETE`/`OPTIONS`), whole-segment `*`/`**` `paths`, `injection` types (`header`, `basic` requiring `username:password` with no `name`, `cookie`, `query`), 1–5 `${SECRET}` template validation, and forbidden routing/connection headers.
- Split RBAC holds on real OpenFGA + scoped API keys (`KeySandboxesHistory` for reads, `KeySandboxesCredentials` for writes): Admins create/delete/restore and change `domain`/`value`; Developers update `injection`/`methods`/`paths`/`enabled` only and get `403` on `domain`/`value`/create/delete/restore; Viewers read metadata only.
- Secret values live in OpenBao only, are never stored in Postgres, never returned by any endpoint (even to Admins), never logged, and are purged on credential delete, version pruning (past current + 3 most recently active distinct prior configs), sandbox-group delete, and workspace teardown.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w2` (2026-11-18), materialized via `/pmx them all to w2`. Reopens [ADR104 § Out of scope](../../../../docs/ADR104-sandbox-render-parity.md#out-of-scope) under Render parity first; closes the 7 sandbox-group-credential omissions in [ADR018 § Render operation omissions](../../../../docs/ADR018-render-parity.md#render-operation-omissions) and `lego/backend/internal/api/render_operation_coverage_test.go`.
- **Goal linkage:** Render parity first ([ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first)) — closes the last non-excluded omitted operation family in Render's 264-operation Public API.
- **Expected outcome:** REST clients, `bex sandbox-groups credentials`, and Render's Python SDK can create, list, retrieve, update, version, restore, and delete sandbox group credentials with full OpenAPI conformance and zero secret disclosure.
- **Why now:** Sandboxes already shipped groups (`w22/m311`, `w2/m359`), images (`w2/m358`), snapshots (`w32/m285`), network policies (`w32/m283`), and scoped API key permission wiring (`w9/m496`); this milestone is the prerequisite control plane for cross-surface parity (`w2/m533`) and runtime injection (`w2/m534`–`m536`).
- **Render parity:** Included — mounts 7 public Render REST endpoints; GraphQL, MCP, and Dashboard follow in `w2/m533`.
- **Depends on:** nothing.
- **Brainstorm recommendations, not user decisions** (record in the ADR as recommendations resting on ADR008 Render parity first and existing repo architecture; open sign-offs stay flagged): (1) store secret values write-only in OpenBao (matching ADR013/ADR098/ADR112) with Postgres holding metadata, versions, and group-salted SHA-256 fingerprints; (2) tie-break overlapping credentials on the same hostname deterministically by lexicographical `credentialId` (Render's OpenAPI says multiple matching credentials are not combined and does not publish tie-breaking order); (3) runtime injection uses `HTTPS_PROXY` + Sandbox Broker CA in `w2/m534`–`m535` (recording the non-proxy/cert-pinning client difference vs Render's Firecracker transparent TCP-443 interception as a divergence awaiting user sign-off).
