# w2 · m73 — Environment groups: correctness, scoped workflows, and live Render parity

**Worker:** worker2 **Goal:** environment groups never lose generated secrets, accept ambiguous names, or leave stale dashboard state, and their list/detail workflows match Render's safe scoped editing model without weakening bex's secret-handling boundaries **Status:** **DONE 2026-08-17**

## Problem

An authenticated Bex-versus-Render audit on 2026-08-17 found three correctness failures and several connected workflow gaps on the production-shaped Environment Groups surface:

| severity | observed Bex behavior | Render reference |
| --- | --- | --- |
| P0 | Detail → Add variable → Generate → Save sent `SetEnvGroupVar(value: "")`, showed success, and persisted a blank value. The shared editor passed a `generateValue` flag that the group hook and per-key API contract discarded. | A non-empty generated secret is stored and remains masked. |
| P1 | Two groups with the same name were accepted in one workspace. `findGroupByName` then chose the first sorted id, so Blueprint `fromGroup` could bind to the wrong secret set. | Duplicate names are rejected with “Environment group name already exists.” |
| P2 | A successful delete toast appeared while the deleted card remained for more than seven seconds/until reload. The detail route opts out of the list refetch. | The row disappears immediately. |
| workflow | Bex omits Environment scope from its group list/detail query, offers no Move group action, and presents service-link candidates outside the group's Environment. | List/detail expose scope, Move group is available, and incompatible links are not offered. |
| workflow | Group values/files mutate one item at a time, keys cannot be renamed coherently, and every write immediately rolls linked services. | Edit is staged, supports Cancel and batch save intent. |
| workflow | `.env` import, safe copy/download export, per-value copy, Clone, and populated-list search/table metadata are absent. | These are first-class group workflows. |

The audit also confirmed deliberate Bex advantages that must survive: create can pre-link services, group-side link/unlink remains available, and owner/timestamp/link-count metadata stays visible.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Generated group values: preserve `generateValue` end to end — **DONE** | 60m | — |
| t002 | Workspace-scoped unique names + fail-closed legacy ambiguity — **DONE** | 75m | — |
| t003 | Delete/cache convergence + truthful rollout notices — **DONE** | 35m | — |
| t004 | Surface Environment scope and constrain service-link candidates — **DONE** | 50m | — |
| t005 | Move-group workflow with atomic membership validation — **DONE** | 45m | t004 |
| t006 | Sparse staged group-content patch + explicit rollout modes — **DONE** | 75m | — |
| t007 | Masked view/edit group editor with Cancel and key/file rename — **DONE** | 60m | t006 |
| t008 | `.env` import + fail-closed copy/download export — **DONE** | 50m | t007 |
| t009 | Clone Environment Group across authorized workspace/Environment targets — **DONE** | 60m | t004, t006 |
| t010 | Searchable Render-shaped group list/table — **DONE** | 45m | t004 |
| t011 | Render parity: authenticated full-surface acceptance + durable evidence — **DONE** | 50m | t001, t002, t003, t005, t008, t009, t010 |
| t012 | Simplify — **DONE** | 30m | t011 |
| t013 | Test coverage and repository gates — **DONE** | 60m | t011, t012 |
| t014 | Closeout — **DONE** | 15m | t013 |

## Definition of done

- Generate on an existing group stores a cryptographically generated, non-empty value through dashboard, REST, GraphQL, and MCP without returning secret material in ordinary mutation/list responses.
- Exact trimmed names are unique within one workspace on create and rename, including concurrent requests; the same name remains valid in different workspaces. Legacy duplicates are reported and every name-based resolver fails closed until an operator resolves them.
- Delete evicts the group from Apollo state before navigation/success completion; no deleted card survives until reload. Rollout copy is shown only when linked services were actually affected.
- List and detail show Environment scope; Move group preserves unrelated Environment memberships; only compatible services can be linked and the server remains the final enforcement boundary.
- Group env vars and files open masked/read-only, enter one discardable draft, and commit as one sparse revision-aware patch. Opaque unchanged values survive, keys/files can be renamed, generation is supported, and the selected save mode causes zero or one rollout/rebuild per linked service—never one rollout per row.
- Users can import dotenv text/file, copy one value, copy all variables, download `.env`, clone a group to an authorized workspace/Environment, and live-search the populated list with Render-shaped columns and empty/no-match states.
- The authenticated Render/Bex acceptance matrix is captured without persisting secret values, deliberate Bex advantages remain intact, ADR018 is truthful, relevant backend/dashboard suites and lint/typecheck pass, and synthetic audit data is removed.

## Source + Goal linkage

- **Source:** user-directed authenticated live audit of `dashboard.bex.co/env-groups` against `dashboard.render.com/env-groups` on 2026-08-17. The audit reproduced the blank generated secret, same-workspace duplicate acceptance, and stale-delete card; then walked Render's populated list, detail Edit, Move group, Import from `.env`, Export, Clone, and search flows. Synthetic groups and the temporary browser profile were deleted after the audit. The code seams were localized to `dashboard/src/features/services/components/env-vars-panel.tsx`, `dashboard/src/features/env-groups/`, `dashboard/src/routes/env-groups_.$groupId.tsx`, and `lego/backend/internal/envgroups/`.
- **Goal linkage:** pillar 1 / Render compatibility (`docs/ADR008-vision.md`, `docs/ADR018-render-parity.md`) plus core secret correctness. A success toast must never mask lost secret data, name-based Blueprint resolution must be deterministic, and Render-trained users must be able to manage shared configuration without accidental per-row deployments.
- **Expected outcome:** environment-group operations are deterministic and fail closed; the dashboard converges immediately after mutations; Environment scope is visible and enforced; multi-variable edits become reviewable, cancelable, and one-roll; import/export/clone/search close the observed live-product gaps.
- **Why now:** the P0 and P1 findings can silently deploy the wrong or empty credential, while the existing ADR018 row currently overstates parity and documents immediate-roll behavior as the only remaining workflow divergence. The live audit provides a bounded, current target and the service Environment editor from `w5/m44` supplies reusable staged-draft, dotenv, generation, and export primitives.
- **Standing closing tasks:** t011 Render parity, t012 Simplify, t013 Test coverage, and t014 Closeout are included because the milestone changes Core plus REST/GraphQL/MCP/dashboard contracts.

## Guardrails

- Never reveal values during list/get, draft initialization, search, scope moves, or clone previews. A bulk export is all-or-nothing after fresh sensitive authorization.
- Do not silently merge or delete legacy duplicate groups. Detect, report ids/workspace/name without values, and make name resolution return a coded ambiguity error.
- Keep legacy item-level write endpoints for compatibility; route them through the corrected primitives and preserve their documented immediate-roll default unless the request explicitly carries a supported save mode.
- Keep Bex's group-side link/unlink and create-time `serviceIds`; do not regress them merely to copy Render's information architecture.
- Do not add Render's datastore-URL picker; `.pm/DO_NOT_DO.md` keeps that workflow out of scope.

## Completion notes

- **Generated values and secret safety:** create, legacy item writes, and sparse patches share one literal-or-generate validator; generation occurs in Core. Ordinary list/get/mutation/clone results carry only keys, file names, metadata, and opaque revisions. Fresh per-value/file reveal remains the only plaintext read.
- **Names and transactions:** exact trimmed names are CAS-unique inside a workspace, create/rename/delete compensate owned claims, and legacy ambiguity returns `ENV_GROUP_NAME_AMBIGUOUS`. `BEX_ENV_GROUP_NAME_CLAIM_AUDIT=dry-run|apply` reports or backfills only unambiguous claims. One group revision serializes sparse env+file changes, compensation, and rollout-only retry; save-only/deploy/rebuild fan out at most once per linked service.
- **Scope and workflows:** Environment scope is exposed and batch-loaded, Move and linking are server-validated, Clone copies contents server-side without links or plaintext, and the dashboard ships masked staged Edit/Cancel, key/file rename, dotenv import, generated values, safe copy/download, compatible linking, searchable table, and immediate delete convergence. Bex's create-time/group-side linking remains intact.
- **Live evidence:** the authenticated Render reference and the final production-build Bex browser pass are recorded in `docs/render-artifacts/env-groups-live-parity.md`. The pass covered P0/P1/P2 plus scope/move/link, staged save/Cancel, import/export, clone, search, and cleanup.
- **Simplify:** the full diff received three parallel `/simplify` reviews. Accepted changes centralized opaque revisions, map cloning, Project/Environment mapping, dotenv upsert, pending-choice logic, and codegen config; removed redundant source refetches/projection work; and replaced the scope-index N+1 with one workspace Environment query. Authorization, secret custody, and rollout boundaries remain explicit.
- **Gates:** `go test ./...`, `go build ./...`, `make lint` (all four Go modules), dashboard `yarn lint`, `yarn typecheck`, `yarn test` (325 files / 2,223 tests), and the production `yarn build` passed. The four high-risk env-group concurrency/compensation tests passed under `go test -race -count=20`. Offline schema dump + GraphQL codegen was byte-identical.
- **Cleanup:** every synthetic group/service/Environment/Project/Workspace/identity and tenant namespace was removed. The test OpenBao release/namespace and temporary JWT/config were deleted; the operator was restored at 1/1 Ready. No follow-up remains inside m73's scope.
