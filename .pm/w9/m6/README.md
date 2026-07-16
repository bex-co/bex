# w9 · m6 — Key Value stable `red-` id + rename

**Worker:** worker9 **Goal:** Key Value stores get a stable Render-shaped `red-…` id and a mutable display name, mirroring what w9/m3 shipped for Postgres — closing the last documented name-as-id datastore deviation. **Status:** code-complete — t001–t005, t007–t009 done; **t006 (live dev/prod official-CLI rollout) remains**, blocked on live cluster access; t010 closeout follows t006.

## Tasks (in order)

| id   | title                                                                          | est | depends_on | status |
| ---- | ------------------------------------------------------------------------------ | --- | ---------- | ------ |
| t001 | Capture Render's Key Value id/rename contract (OpenAPI + live docs)            | 30m | —          | — **DONE** |
| t002 | Mint `id.Kind` keyvalue (`red-` prefix) + CR identity + backfill migration     | 45m | t001       | — **DONE** |
| t003 | Store + projector: stable id column, mutable name                              | 30m | t002       | — **DONE** |
| t004 | REST/GraphQL/MCP: route by `red-` id + rename verb                             | 45m | t003       | — **DONE** |
| t005 | Dashboard: rename control + id display                                         | 30m | t004       | — **DONE** |
| t006 | Roll dev-N + prod; official-CLI `red-` routing verify leg                      | 45m | t005       | todo (live infra) |
| t007 | Render parity                                                                  | 30m | t006       | — **DONE** |
| t008 | Simplify                                                                       | 30m | t007       | — **DONE** |
| t009 | Test coverage                                                                  | 45m | t007       | — **DONE** |
| t010 | Closeout                                                                       | 15m | t009       | todo (after t006) |

## Definition of done

A Key Value store carries a stable `red-…` id that survives rename; the unmodified official Render CLI routes to it by id; `docs/cli-compatibility-checklist.md` RC5 flips to ✅ and `docs/ADR018-render-parity.md:218`'s `red-` routing divergence is recorded closed with evidence.

## Progress (2026-07-15)

Code-complete and test-verified across all four first-party surfaces; the one remaining item is the live official-CLI dev/prod rollout (t006), which needs the operator's live cluster environments.

- **Identity split (t002/t003):** `id.KeyValue` (`red-` prefix) registered in `lego/backend/internal/id`; `KeyValueSpec.Name` + `ValidKeyValueName` + `KeyValue.DisplayName()` added to `lego/types/v1alpha1/keyvalue_types.go`; CRD regenerated. `CreateKeyValue` mints `red-<xid>` as `metadata.name` and stores the request name in `spec.name`; `kvView` returns `ID=metadata.name`, `Name=DisplayName()`; workspace-scoped display-name uniqueness (`ensureKeyValueNameAvailable`). The bex.yml/deploy-from-chat stack path (`internal/apps/deploy.go applyKeyValue`) now resolves by display name and mints `red-` ids too, matching `applyDatabase`.
- **Rename verb (t004):** `KeyValuePatch{Name,Plan}` + `UpdateKeyValue`/`PreviewUpdateKeyValue`; REST `PATCH /v1/key-value/{id}` accepts an all-pointer `{name,plan,dryRun}` body (a name-only PATCH no longer 400s); GraphQL `renameKeyValue`; MCP `rename_key_value` (bex extension, sibling of `rename_postgres`). One core backs all three.
- **Dashboard (t005):** `KeyValueNameSection` rename card + read-only `red-` id row on `keyvalue.$keyValueId`; `useRenameKeyValue` hook + `RenameKeyValue` mutation; en/zh locales; component/hook tests. `yarn typecheck && yarn lint && yarn test` green (1169 tests).
- **Migration + verify scripts (t002/t006):** `scripts/keyvalue-name-migrate.sh` (idempotent, non-destructive `spec.name` backfill) + `keyvalue-name-migrate.test.sh` (green), `keyvalue-rename-verify.sh` (identity snapshot/compare), `keyvalue-rename-cli-smoke.sh` (official-CLI rename leg).
- **Docs (t007):** ADR020 registry + Known deviations, ADR021 §6 (identity & rename), ADR018 ledger + divergence row, `cli-compatibility-checklist.md` (`keyvalues update --name` row + verification section), `docs/render-artifacts/key-value-identity.md` (t001 pinned contract). Divergence + checklist rows are marked ◐ pending the live rollout — not falsely closed.
- **Tests (t009):** backend `go test ./...`, operator `make test`, backend lint, and the migration test all pass. New backend coverage: REST/GraphQL/MCP rename, dry-run, workspace-scoped duplicate rejection, cross-workspace name reuse, id-stable-across-rename, and `?name=` resolves new-not-old.
- **Simplify (t008):** changes are a direct mirror of the already-simplified Postgres sibling; manual review found no behavior-preserving reduction to apply (the `$simplify` skill was not run).

**Remaining (t006 → t010):** run `keyvalue-name-migrate.sh` then `keyvalue-rename-cli-smoke.sh` against dev-1…dev-10 and production with the pinned official CLI, capture before/after identity evidence, then flip the checklist/ADR018 rows to ✅ and move the milestone to `w9/done/m6/`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 12, 2026-07-15 — closeout-residual sweep (`w9/done/m3/README.md:41` left KeyValue explicitly out of scope; checklist RC5 calls it "the one documented name-as-id datastore deviation").
- **Goal linkage:** Render parity across all five surfaces, incl. the official CLI (fifth surface, w9/m2 charter).
- **Expected outcome:** the last name-as-id datastore deviation is gone; CLI `red-` client-side routing works against bex.
- **Why now:** the identical Postgres migration (w9/m3) is freshly shipped — pattern, scripts, and rollout playbook are warm; w9's other open milestones are small. Render parity closing task included — this touches REST/GraphQL/MCP/UI.
