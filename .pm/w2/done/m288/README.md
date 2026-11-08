# w2 · m288 — Blueprint sync ids answer as `exe-`, Render's prefix

**Worker:** worker2 **Goal:** every client-read surface shows a Blueprint sync as `exe-<xid>`, as Render does, and every sync-id input accepts `exe-` or the durable `bsr-`. Storage stays `bsr-`. **Status:** done (2026-11-08)

## Tasks (in order)

| id   | title                                                                                      | est  | depends_on             |
| ---- | ------------------------------------------------------------------------------------------ | ---- | ---------------------- |
| t001 | `id.BlueprintSyncPublicID` / `BlueprintSyncStorageID` on the shared `respell` helper — **DONE**        | 20m  | —                      |
| t002 | REST (Render-shaped create/plan/approve bodies, list-blueprint-syncs), GraphQL and MCP emit `exe-`; inputs accept both — **DONE** | 1.5h | t001 |
| t003 | Dashboard Sync History and links use the API's id — **DONE**                                          | 30m  | t002                   |
| t004 | Amend ADR020 (sync-id line) and the OpenAPI path gate tests for both spellings — **DONE**             | 30m  | t002                   |
| t005 | Render parity — **DONE**                                                                              | 15m  | t003, t004             |
| t006 | Simplify — **DONE**                                                                                   | 15m  | t005                   |
| t007 | Test coverage — **DONE**                                                                              | 30m  | t005                   |
| t008 | Closeout — **DONE**                                                                                   | 10m  | t006, t007             |

## Definition of done

- REST, GraphQL, MCP and the dashboard show Blueprint sync ids as `exe-…`. Any sync-id input given `bsr-…` still resolves to the same sync.
- Stored rows and labels are unchanged; nothing is migrated.
- ADR018's "Blueprint and Blueprint sync id prefixes" row is ✅, and ADR020 records the public/durable split.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2`, 2026-11-08, item 1.
  - ADR018 lists the sync-id prefix (`bsr-` vs Render's `exe-`) as a known deviation.
  - ADR020 calls it "unverified rather than confirmed safe" only because no pinned path parameter constrains a sync id yet.
  - w2/m269 showed the cost of waiting: CLI v2.35 began validating Blueprint ids client-side and refused every bex id before sending a request.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first).
- **Expected outcome:** Render tooling that matches `exe-` sync ids works unchanged, and the next CLI pin bump cannot break sync commands this way.
- **Why now:** do it before the next pin bump rather than after a refusal. m269 already built the pattern (`respell`, `canonicalLinkId`).
- **Render parity included:** REST/GraphQL/MCP/UI id surface.

## Closed — 2026-11-08 (`/loopx w2`)

- **Ids:** `id.BlueprintSyncPublicID`/`BlueprintSyncStorageID` (`exe-` ⇄ `bsr-`) use the shared `respell`. `exe` is also `SandboxExecution`'s prefix, as on Render. The two cannot cross, because only sync-id inputs are respelled; this is pinned by a test.
- **Emitted as `exe-`:** sync views, `list-blueprint-syncs` rows and cursor, `sync.id` in the Render-shaped create/plan bodies, the approved-sync `id`, the approve refusals' `syncId` (REST, GraphQL, MCP), GraphQL `blueprintPlannedSync`/`blueprintSyncs`, and MCP `list_blueprint_syncs`/`update_blueprint`.
- **Input:** the history cursor, the only sync-id input, accepts both spellings.
- **Unchanged:** storage, `active_run_id` and alert keys stay `bsr-`.
- **Render-conformance allowlist:** both "Blueprint and Blueprint sync id prefixes" entries are retired (108 → 106), and full schema conformance now holds for GET syncs.
- **Docs:** ADR020 and ADR018 updated.
- **Tests:** `TestBlueprintSyncAliasContract`, `blueprint_sync_public_id_test.go` (every surface; it fails if the cursor conversion is removed) and `TestBlueprintSyncSpellingsMatchRendersPattern`. The real-DB apps/store/api suites, `make lint`, and dashboard lint and vitest all pass.
- **Dashboard:** keys rows by the API's id and builds no sync links, so no product change was needed.
