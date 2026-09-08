# w2 · m91 — Repair the three Divergent MCP tools (w1/m70's filed repairs)

**Worker:** worker2 **Goal:** `create_postgres`, `create_static_site`, and `get_metrics` accept Render's exact upstream argument contracts, and the MCP parity inventory reclassifies them out of `Divergent`. **Status:** todo

## Tasks (in order)

| id   | title                                                                                          | est | depends_on         |
| ---- | ----------------------------------------------------------------------------------------------- | --- | ------------------ |
| t001 | `create_postgres`: accept Render's `diskSizeGb` casing; decide the alias policy for all three   | 30m | —                  |
| t002 | `create_static_site`: wire `autoDeploy` + `buildCommand` to the existing `CreateRequest` fields | 30m | —                  |
| t003 | `get_metrics`: adopt Render's arg names; triage the 7 missing upstream args                     | 45m | t001               |
| t004 | Flip the parity-inventory classes; update ADR018 + the m70 evidence note                        | 20m | t001, t002, t003   |
| t005 | Simplify                                                                                        | 20m | t004               |
| t006 | Test coverage                                                                                   | 30m | t004               |
| t007 | Closeout                                                                                        | 10m | t006               |

## Definition of done

The three tools accept Render's exact upstream argument names — proven by tests that call each with Render's spellings (`diskSizeGb`, `autoDeploy`, `buildCommand`, `resourceId`, `httpLatencyQuantile`, `resolution`). Legacy bex spellings keep working per the alias policy t001 decides (or their removal is recorded with rationale). `get_metrics`' 7 missing upstream args are each wired or rejected with a recorded reason. `TestMCPParityInventory` reflects the reclassification (the three leave `Divergent` for `Superset`/`Parity1to1` as measured) and stays fail-closed against regrowth. ADR018 line 11's "filed for repair" sentence is replaced with the done record, and `w1/done/m70/evidence/inventory.md` rows 46–48 are annotated repaired. Cross-surface consistency holds: REST/GraphQL spell the same facts per Render and did not drift (verified in t004). Backend `go test ./...` + `make lint` green.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-09-07 #4. `w1/done/m70/evidence/inventory.md` rows 46–48 marked these three tools "**repair**" (unintended contract breaks, unlike the four accepted genuine differences); ADR018 line 11 says "filed for repair" — but no open board item existed, so the filing evaporated when m70 closed. All three verified still wrong in code: `lego/backend/internal/postgres/mcp.go:62` (`diskSizeGB` casing), `lego/backend/internal/apps/mcp.go:449-466` (`createStaticSiteArgs` has no `autoDeploy`/`buildCommand`), `lego/backend/internal/metrics/mcp.go:33-39` (`resource`/`quantile`/`resolutionSeconds` renames).
- **Goal linkage:** Render MCP parity (ADR006/ADR018) — tools sharing an upstream name while breaking its argument contract give agents that follow Render's documented MCP schema validation errors or silently dropped fields.
- **Expected outcome:** an agent driving bex through Render's documented MCP contract succeeds with Render's exact spellings on all three tools; the inventory guard makes the repair permanent.
- **Why now:** these are the only *unintended* contract breaks the m70 pin found; ADR018 publicly promises their repair; cheap, bounded, and unowned.
- **Render parity omitted (standing closing task):** redundant — the milestone *is* the parity repair (the w2/m79 precedent); the cross-surface REST/GraphQL consistency check it would perform is folded into t004's acceptance criteria instead.
