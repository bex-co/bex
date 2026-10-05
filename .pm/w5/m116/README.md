# w5 · m116 — Dry-run is the real call stopped before its first write, for create and update of every kind

**Worker:** worker5 **Goal:** Every `dryRun` or preview runs exactly the real call's read-only checks (billing, protection, names, hosts, registry, quota, CRD admission) because both run the same plan step. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Inventory every preview path and decide the preview relation | 30m | — |
| t002 | Give apps create and update one plan step | 1h | t001 |
| t003 | Give Postgres and Key Value one plan step and delete the Preview twins | 1h30m | t002 |
| t004 | End the plan with a server-side dry-run create | 45m | t003 |
| t005 | Render parity | 20m | t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 45m | t005, t006 |
| t008 | Closeout | 10m | t007 |

## Definition of done

- For service, Postgres and Key Value create, update and plan change, a dry-run returns exactly the real call's error, and writes nothing, for each of:
  - the count cap;
  - an unpaid plan with billing enforced;
  - a protected environment;
  - a domain held by a pending-verification host;
  - a CRD-invalid field.
- The five `Preview*` functions are gone, and every billing check goes through `RequirePlanBilling`.
- `docs/ADR006-bex-api.md` no longer says count caps aren't previewed, and the stale `apps/service.go` comment claiming `create` calls `createNewApp` is fixed.

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05, of w8/045 (`10827a8cd`) and w8/046 (`abe8f0eb5`). Both fixed create dry-run drift by copying checks. The update side still drifts: `PATCH /v1/services/{id}?dryRun=true {"plan":"standard"}` answers 200 where the real call answers 402. The store-mode host preview also misses pending-verification domains (preview 200, real 409).
- **Goal linkage:** ADR006/ADR018 Render-compatible `dryRun`; ADR046/ADR075 billing gates.
- **Expected outcome:** A successful dry-run means the real call passes its checks.
- **Why now:** The class recurred twice in a day, the update side is still open, and w5/m117 and w5/m118 build on the plan seam.
- **Render parity included:** dry-run answers change on REST/GraphQL/MCP.
- **Depends on w5/072:** the minimal Blueprint card-gate fix lands first.
