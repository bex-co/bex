# w4 · m178 — Refresh source project membership after a move

**Worker:** worker4 **Goal:** a completed project move removes the resource from the source project's row and count without a reload, while preserving authoritative target placement and existing navigation caching. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Refresh the source Project before publishing the loader snapshot | 30m | — |
| t002 | Audit the shared move family and preserve passing refresh controls | 20m | w4/m178/t001 |
| t003 | Render parity | 15m | w4/m178/t002 |
| t004 | Simplify | 10m | w4/m178/t003 |
| t005 | Test coverage with real Router and Apollo cache | 40m | w4/m178/t003, w4/m178/t004 |
| t006 | Closeout | 10m | w4/m178/t005 |

## Definition of done

Repeat the owned Free image-web journey in [finding.md](finding.md), creating fresh qa-prefixed fixtures and cleaning them afterwards:

- From a fresh source project's Unassigned page, Actions → Move to project → the other owned project succeeds. Once its refresh settles, the source row and Services count are zero without reload, and GraphQL shows source empty / target assigned. Repeat in the opposite direction.
- Reload of the source agrees with its already refreshed display; it is no longer needed to remove the row.
- Remove from project still updates the source table without reload, leaves the service ungrouped, and does not perform an extra membership write.
- The fixture keeps Running revision 1 and its public marker returns HTTP200 through the placement-only operations. No unrelated resource is moved.
- Delete the service/projects/environments, verify logical REST404 and no owned cluster objects, and revoke this QA session. Only mark done after the live replay passes.

Sibling kinds, Overview, refresh-rejection and prefetch controls were not exercised by this live finding; t002/t005 verify them without representing them as already observed failures.

## Source + Goal linkage

- **Source:** continuous qa-find-bugs, 2026-10-07 UTC sweep 8, muse.env, workspace bex; [researched finding and complete probe](finding.md). Research main 2892c2091.
- **Goal linkage:** ADR008 dependable hosting; ADR032 truthful Project/Environment placement.
- **Expected outcome:** successful moves change the displayed membership automatically, with unchanged backend placement and runtime.
- **Why now:** the old source-refresh guarantee from bb2ce9d74 regressed when retained loaders became cache-first. The UI toast reports the move while the old project still claims the resource; repeated live captures isolate that disagreement.
- **Sizing:** ~2h 5m across a bounded fix, three-kind family audit, real Router/Apollo integration regression, parity, simplification and live closeout. The integration must reproduce cache publication ordering; the existing mocked invalidate-call tests do not cover it.
- **Render parity:** included (t003), because displayed project membership changes. REST/GraphQL/MCP remain canonical; compare the UI to those reads and the documented Render move surface without inventing a different grouping model.
- **Scope:** project refresh only. Keep the shared title policy's 16 call sites in 15 files and the single target membership write.
- **Unverified:** sibling resource-kind live moves, Overview rendering, refresh rejection, other title-loader consumers, MCP and authenticated Render timing; see finding.md.
