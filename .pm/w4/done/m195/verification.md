# m195 verification

Shipped in `1cc56f66a` (`fix(env): save an opaque rename into an explicitly deleted name (w4/m195)`); deploy run 38115901019 succeeded and the platform pin `85a0f92f2` followed. Live acceptance ran 2026-10-11 UTC on dashboard.bex.co as the QA user, workspace `tea-d98210cbbpdc73dcrkvg`.

## Change

- `lego/backend/internal/core/environmentpatch.go`: the one undifferentiated `seen` set became a per-name `nameUse` (written / deleted / consumed). `nameUse.admits` lets exactly one opaque rename from a distinct source follow an explicit delete of its destination. Rename-first onto an occupied name, cycles, delete+recreate with a payload, duplicate deletes/writes, a rename from a deleted name, repeated/missing sources and rename+payload keep their refusals. The `nameConsumed` state is where w4/234 can admit a payload write onto a vacated rename source.
- `dashboard/src/features/services/lib/environment-draft.ts`: `patchRows` emits stored-row deletions before every other operation (readonly rows still contribute nothing; changed-content rename pairs stay delete-old + write-new in row order).

## Caller audit (t003)

- Production helper calls: `secrets/batch.go:538,542` (service sparse batch + CAS) and `envgroups/patch.go:187,190` (group sparse/CAS). All apply to a cloned/current map inside a validate-before-commit step; a refused batch writes nothing (service sparse already undoes the env write if the files half refuses).
- Service CAS (`validateCASPatch`) still admits only one ordinary value update: no rename/delete, so the new case cannot reach it. Group `expectedRevision` is unchanged and advanced once per admitted save.
- The six adapters (service REST `PATCH /v1/services/{id}/environment`, GraphQL `patchServiceEnvironment`, MCP `patch_service_environment`; group REST `PATCH /v1/env-groups/{id}/contents`, GraphQL `patchEnvGroupEnvironment`, MCP `patch_env_group_environment`) decode arrays in request order with no sorting, so all reach the same core rule. Group `/environment` (scope move) is untouched.
- One dashboard producer (`service-environment-editor.tsx:337`) serves the service page (all service types via `/$serviceId/env`) and the group editor.

## Live receipts

Fixtures: Free BusyBox web service `srv-db5iorilrmtc73e3jd5g` (qa-20261010-m195-renames) and unlinked group `evg-db5ipoaf896c73b2t590` (qa-20261010-m195-group).

| at (UTC) | surface | case | result |
| --- | --- | --- | --- |
| 06:28:51 | UI desktop | ordinary opaque rename LEFT→NEW, Save only | 200, `rolledOut:false` |
| 06:29:0x | GraphQL | occupied cycle RIGHT⇄NEW | `rename destination "QA_M195_RIGHT" already exists`; store unchanged |
| 06:29:28 | UI desktop | combined: delete RIGHT/right, rename NEW→RIGHT/new→right (NEW sorted first) | request delete-first; 200; store RIGHT=`left value\n`, right=`left-file\n`, NEW/new absent; deploys unchanged (1), `undeployedChanges:true`; public body still old bytes |
| 06:29:43 | UI | Manual Deploy → Deploy latest image | Live, `undeployedChanges:false`; public body RIGHT hex `6c6566742076616c75650a`, right hex `6c6566742d66696c650a`, LEFT/NEW/left/new absent |
| 06:31:09 | UI group | mixed: delete DEST, rename SOURCE→DEST (DEST sorted first) | 200; revision `egr1_AAAAAAAAAAE`→`egr1_AAAAAAAAAAI`; links empty; DEST=`source value\n`, dest=`source-file\n` |
| 06:31:26 | UI group | file-only dest2/source2 | 200; revision →`egr1_AAAAAAAAAAM`; dest2=`source2-file\n` |
| 06:31:34 | REST group `/contents` | variable-only DEST2/SOURCE2 | 200; revision →`egr1_AAAAAAAAAAQ`; DEST2=`source2\n` |
| 06:32:03 | UI desktop | variable-only | 200; RIGHT=`var only\n` |
| 06:32:22 | UI 390×844 fresh load | file-only | 200; right=`file only\n`; draft settled; document/body width 390 |
| 06:32:4x | REST service | combined delete-first | 200; RIGHT=`rest var\n`, right=`rest-file\n`; no new deploy |
| 06:33:xx | UI desktop | swap of two occupied names | refused with the real `Environment variable rename destination "QA_M195_RIGHT" already exists` toast; draft retained; store unchanged |

No Save-only step minted a deploy (deploy count stayed 1 until the explicit deploy, then 2).

## Render parity (t004)

Render's public API edits environment variables per key (`PUT/DELETE /services/{id}/env-vars/{key}`, `PUT /services/{id}/env-vars` replace-all) and secret files likewise; it has no ordered sparse batch with opaque renames. The Bex mixed batch is an extension; Render per-key routes are unchanged. No authenticated Render replay was performed. Unverified: MCP writes live (same core path as REST/GraphQL), private/worker/cron/static service types live (same producer and handler), group fanout to linked services, concurrent revision races.

## Tests (t006)

- `core/environmentpatch_test.go`: `TestOpaqueRenameRefillsAnExplicitlyDeletedName`, `TestDeletedNameRefillKeepsItsRefusals` (10 refusal vectors). On the pre-fix helper the refill test fails; 7 refusal vectors pass (pinning that the refusals predate and survive the fix) and 3 report a different refusal message, since the old duplicate check fired first.
- `secrets/batch_test.go` and `envgroups/patch_test.go`: `TestPatchEnvironmentRenameRefillsAnExplicitlyDeletedName` (service sparse save-only with rename-first refusal leaving the store untouched; unlinked group revision advancing once).
- `environment-draft.test.ts`: both stored row orders produce delete-first for variables and files.
- Suites: backend `go test ./...` plus `ship-gates.sh` real-deps run, `make lint` (0 issues), dashboard `yarn lint` + `yarn test` (4505 passed).

## Cleanup

Service and group deleted at ~06:34 UTC; inventories no longer list either id. QA Kratos session revoked via `qa-login.sh --logout` (API then returned 401); browser cookies cleared; local state/jar removed.
