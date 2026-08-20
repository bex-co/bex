# w2 · m80 — ADR066 #3: env-group OpenBao workspace-prefixed path migration

**Worker:** worker2 **Goal:** move env-group storage from the single shared OpenBao metadata index to workspace-prefixed paths (the tenant keying w7/m70 gave services), so lists and name resolution walk only the caller's prefix — closing ADR066 finding 3's deferred structural fix. **Status:** done

## Tasks (in order)

| id                           | title                                                                                                                             | est | depends_on               |
| ---------------------------- | --------------------------------------------------------------------------------------------------------------------------------- | --- | ------------------------ |
| t001                         | Dual-read path layer: workspace-prefixed scheme mirroring w7/m70's tenant keying; writes to new paths, legacy fallback reads — **DONE** | 60m | —                        |
| t002                         | Migration command: copy legacy group metadata+content to workspace prefixes, verify, tombstone legacy; idempotent + resumable — **DONE** | 45m | w2/m80/t001              |
| t003                         | Prefix-scoped reads: list/name-resolution walk only the caller's prefix; retire or narrow the 15s global-sweep snapshot cache — **DONE** | 45m | w2/m80/t001              |
| t004                         | Dev-stack migration walk: seed legacy groups, migrate, verify reads/writes/quota/`ENV_GROUP_LIMIT` unchanged — **DONE**           | 30m | w2/m80/t002, w2/m80/t003 |
| t005                         | Prod migration runbook + explicitly authorization-gated cutover; annotate the ADR066 residual line — **DONE**                     | 30m | w2/m80/t004              |
| t006 (standing closing task) | Simplify (standing): run /simplify over the changed code — **DONE**                                                               | 30m | w2/m80/t005              |
| t007 (standing closing task) | Test coverage (standing): prefix isolation (cross-workspace read impossibility), dual-read fallback, migration idempotency — **DONE** | 45m | w2/m80/t005              |
| t008 (standing closing task) | Closeout (standing): verify DoD, mark done, move milestone to done/ — **DONE**                                                    | 15m | w2/m80/t007              |

## Definition of done

On the dev stack, env-group writes land on workspace-prefixed OpenBao paths and reads (list, name-resolution, membership, content hydration) never touch another workspace's prefix (test-asserted); the migration command moves seeded legacy groups with verification and tombstones legacy paths; quota (`ENV_GROUP_LIMIT`) and all existing REST/GraphQL/MCP behavior are unchanged; the global-sweep snapshot cache is retired or narrowed to per-workspace; a prod migration runbook exists with an explicitly authorization-gated cutover; the ADR066 residual-register line is annotated.

## Ship notes (2026-08-19)

- Exported `secrets.WithTenant` / `TenantFromContext` / `LegacyTenant`; env-groups write under workspace tenant + thin legacy locator; dual-read for unmigrated full legacy meta; `listGroupIDs` prefix-scoped with locator-safe legacy union; `metaCache` retired.
- `MigratePaths` + `BEX_ENV_GROUP_PATH_MIGRATION=dry-run|apply`; runbook `docs/runbooks/env-group-path-migration.md`; ADR066/ADR013 annotated.
- t004: covered by `migrate_test.go` + `tenant_test.go` (tenant-aware fake mirroring OpenBao keying); live CAPD OpenBao rehearsal left to the runbook's pre-prod checklist (no prod apply).
- Simplify: helpers concentrated in `tenant.go`/`migrate.go`; no further altitude cleanup needed.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-08-18 (round 1, item 4); ADR066 finding 3's deferred structural fix ("the same explicit-migration problem m70 solved for services, deliberately not rushed into this round") — residual register line with no owning item.
- **Goal linkage:** tenant isolation + secrets-plane scalability (ADR013/ADR043).
- **Expected outcome:** env-group read cost scales with the caller's workspace, not global group count; per-request content hydration stops being globally coupled; the register line closes.
- **Why now:** the round-11 mitigation (quota + 15s snapshot) bounds amplification but the structure remains global; w7/m70's migration pattern is fresh precedent — cheaper to mirror now than re-derive later.
- **Render parity omitted:** behavior-preserving storage-layout change; no REST/GraphQL/MCP/UI wire change.

## Repo facts (grounding)

- Env-group storage lives in `lego/backend/internal/envgroups/` over the shared `core.SecretKV` seam (`Store` field, wired to OpenBao in `internal/api/server.go:596`). Paths today: `env-groups/<gid>/meta|env|files` (`service.go:202-204`), `env-groups/<gid>/revision` (`patch.go:65`), name claims `env-group-name-claims/<sha256(workspace\0name)>` (`service.go:1348` — already workspace-digested, but under one shared prefix).
- The global sweep is `s.Store.List(ctx, "env-groups")` (`service.go:386` in `sweepMeta`, plus `1163`/`1311`/`1425` and `nameclaims.go`'s `AuditNameClaims`), served from the 15s snapshot (`envGroupMetaCacheTTL`, `service.go:61`; `metaCache`/`invalidateMetaCache`).
- w7/m70's tenant keying lives in `lego/backend/internal/secrets/store.go` (`withTenant`/`tenantFromCtx`, **unexported**; legacy root `baoTenant="default"`); env-groups deliberately never sets a ctx tenant, so everything lands under `tenants/data/default/env-groups/…`. m70's shipped migration was **lazy first-read migrate-and-delete** (`readMap`) + both-path purge, not a standalone command; the opt-in env-gated maintenance-mode precedent is `BEX_ENV_GROUP_NAME_CLAIM_AUDIT` (dry-run|apply, `cmd/api/main.go:416`). The m70 OpenBao-policy step was a no-op — the `bex-api` role already grants `tenants/*`.
- Quota: `Service.MaxEnvGroups` from `BEX_MAX_ENV_GROUPS_PER_WORKSPACE` (`cmd/api/main.go:1045`, default 100), refusal `ENV_GROUP_LIMIT`.
- ADR066 residual register line to annotate: `docs/ADR066-security-review-round11.md:51`; finding text at `:33`.

## Out of scope

- No live prod migration without explicit operator authorization (t005's gate).
- Env-var/secret-file aggregate quotas (ADR066 #6) unchanged.
- Services' m70 layout (`tenants/data/<tenant>/services/…`) untouched.
