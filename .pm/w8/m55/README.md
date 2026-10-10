# w8 · m55 — Preserve literal commas in service name filters

**Worker:** worker8 **Goal:** CLI updates and clones resolve an accepted display name containing a comma. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Define encoded name-array semantics and bound the callers | 30m | — |
| t002 | Preserve raw array separators before decoding name elements | 45m | t001 |
| t003 | Audit shared consumers and preserve other filter contracts | 35m | t002 |
| t004 | Verify Render and cross-surface name contracts | 25m | t003 |
| t005 | Simplify the changed code | 15m | t004 |
| t006 | Add meaningful encoding and CLI-resolution regressions | 40m | t004 |
| t007 | Replay both clients, clean up and close out | 35m | t005, t006 |

## Definition of done

- On a fresh owned Free static site, native CLI rename to `qa-<date>-<nonce>-csv,雪%+&=` succeeds. The actual pinned list request encodes its literal comma as `%2C`; the composed production API returns the exact owned ID for that name. Published Bex and unmodified Render `services update <name> --auto-deploy=false --confirm -o json` both exit 0.
- `services create --from <comma-name> --name <fresh-QA-name> --confirm -o json` succeeds in both clients, and each returned clone becomes Live, serves its own expected HTTP 200 page, and preserves the source's static build/publish settings. These are the originally failed commands; success by ID alone does not close the milestone.
- Single literal-comma names, two ordinary names, a mixed array containing a comma name, repeated keys and literal `%2C` names all remain distinguishable. Existing OR behavior, trimming/deduplication, pagination and AND composition with other filters remain correct. Decode each element once. Preserve the other 28 existing QueryList invocations' contracts unless a separately researched defect justifies a change.
- ID selection, direct named deploy listing, omission, no-match and the punctuation-only name control keep their observed behavior. Shared name-filter callers follow their declared contracts without widening resource-name admission rules.
- Delete every owned clone before its source, verify detail/list/public absence, quota convergence and baseline IDs, and attach dated live evidence after the server fix is deployed. Mark all tasks done and move the whole milestone into `w8/done/m55/` only after these outcomes hold.

## Source + Goal linkage

- **Source:** ongoing user-requested `$qa-find-bugs-cli` w8 loop, sweep 51, 2026-10-09 UTC. [Researched finding and durable evidence](finding.md). Filing only; no implementation or ship authorization.
- **Goal linkage:** ADR008 reliable Render-alternative hosting; ADR006 faithful API adapters; ADR018 pinned Render CLI compatibility; completed w8/034 admits these display names, so supported named commands must resolve them.
- **Expected outcome:** an accepted name continues to identify the same service through native update and clone workflows.
- **Why now:** both published Bex and actual unmodified Render fail against production Bex after a successful rename. The endpoint returns HTTP 200 with an empty list for a correctly encoded name. Direct name-path commands and ID controls pass, isolating the list parser.
- **Sizing:** 7 tasks, 3h45m. The parser has 37 production invocations in nine files; nine name-filter sites in seven files need a scoped contract audit and regression coverage.
- **Scope:** server compatibility. Preserve the pinned client, its request bytes, accepted display-name rules and existing field schemas. No launcher workaround, CLI fork, new naming restriction or product-code fix was made by this hunt.
- **Unverified:** live sibling name-filter resources, GraphQL/MCP/UI named inputs, live Render API behavior, generic path/value comma filters and post-fix deployment. They are bounded verification obligations, not passing claims.
