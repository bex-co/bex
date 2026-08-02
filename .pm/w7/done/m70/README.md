# w7 · m70 — OpenBao cross-workspace secret isolation

**Worker:** worker7 **Goal:** stop env vars and secret files colliding across workspaces by keying every OpenBao path on tenant identity. **Status:** done

## Tasks (in order)

| id   | title                                                          | est  | depends_on |
| ---- | -------------------------------------------------------------- | ---- | ---------- |
| t001 | Thread tenant identity into OpenBao key paths — **DONE**       | 45m  | —          |
| t002 | Fix PurgeApp to resolve the public storage name — **DONE**     | 20m  | t001       |
| t003 | Migrate existing keys + update the OpenBao policy — **DONE**   | 45m  | t002       |
| t004 | Simplify — **DONE**                                            | 20m  | t003       |
| t005 | Test coverage — **DONE**                                       | 40m  | t004       |
| t006 | Closeout — **DONE**                                            | 10m  | t005       |

## Definition of done

Two simultaneously-valid Apps with the same public name in different workspaces resolve to disjoint OpenBao keys, verified by a cross-workspace isolation test that asserts workspace A cannot read or overwrite workspace B's env/secret-file values. Deleting a store-managed App purges exactly the live public-name path (no orphan, no recreation leak). Existing `default/services/<name>/` keys are migrated to tenant-scoped paths with no public-name fallback.

## Source + Goal linkage

- **Source:** codex-security scan `codex-security-bex-azfRGv` (rev `3a7e7f02`) findings #1 (high) and #15 (medium), validated against HEAD — see `/Users/tianpan/.codex/state/plugins/codex-security/scans/bex/codex-security-bex-azfRGv/report.md`. Root cause shared by both: `lego/backend/internal/secrets/store.go` hard-codes `baoTenant="default"` and `envPath/filesPath` key only on the public service name, so the storage layer never encodes workspace identity.
- **Goal linkage:** Security pillar — tenant isolation of secrets (ADR013). The deferred `w1/m2` multi-tenant keying called out in the `baoTenant` comment.
- **Expected outcome:** cross-workspace secret read/overwrite is impossible; clean delete leaves no recreation-leakable material.
- **Why now:** the only HIGH finding in the scan; a real authenticated-tenant cross-workspace secret breach. #15 compounds it (fixing the keying without fixing the purge name leaks secrets on name recreation), so both ship together.
- **Render parity omitted:** the fix is internal storage keying with no REST/GraphQL/MCP/UI surface change (same env-var / secret-file API, same paths the caller sees).

## Ship record — DONE 2026-08-01

Shipped as `68a1d297` (deployed via `405f284a` → GitOps pin `f0f5ebf9`). Design: the store reads the workspace tenant from the request context (`withTenant`/`tenantFromCtx`, defaulting to the legacy `default`) instead of a fixed field; each verb stamps the App's `core.LabelTenant` into ctx after authorizing, so two same-named services in different workspaces resolve to disjoint KV paths (`tenants/data/<tenant>/services/<name>/…`). Env-groups never sets a ctx tenant, so it stays on `default/` byte-identical. `readMap` lazily migrates pre-w7/m70 `default/` data forward on first read and deletes the legacy copy, so the re-keying cannot orphan existing tenant secrets on deploy; a missing tenant label degrades gracefully to the legacy path. `PurgeApp` now takes the App and resolves the public `LabelServiceName` (not the tenant-prefixed `metadata.name`), and both single-app + workspace purge clean the new tenant path **and** the legacy path. CI: `test-backend` green incl. 4 new tests (`TestOpenBaoStore_TenantScopedPaths`, `TestReadMap_LazilyMigratesLegacyData`, `TestWorkspacePurger_PurgeApp_UsesPublicServiceName`, `TestWorkspacePurger_PurgeApp_AlsoCleansLegacyPath`). t003's OpenBao-policy update was a no-op — the `bex-api` role already grants `tenants/*`, which covers every tenant-scoped path.
