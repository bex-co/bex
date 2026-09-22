# w1 · m164 — A cancel over a never-served release must not stamp a `releaseGeneration` that never succeeded

**Worker:** worker1 **Goal:** a first release that never served (a crash loop) followed by a cancel closes its deploy row truthfully, with no generation recorded as successful, while Apps that predate `status.activeRevision` keep reporting exactly what they do today. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                      | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Backend evidence: what the deploy reconciler does with `releaseGeneration` 0 and with legacy Apps lacking `activeRevision` | 45m | —          |
| t002 | Operator: a never-served release reports no successful generation; the legacy fallback becomes an explicit branch          | 40m | t001       |
| t003 | Reconciler test for both shapes plus an envtest for crash-loop-then-cancel                                                  | 40m | t002       |
| t004 | Render parity                                                                                                              | 20m | t003       |
| t005 | Simplify                                                                                                                   | 15m | t004       |
| t006 | Test coverage                                                                                                              | 40m | t004       |
| t007 | Closeout                                                                                                                   | 10m | t006       |

## Definition of done

- A canceled deploy over a never-served first release closes as `canceled` and `status.releaseGeneration` records no successful generation (0, or the value t001's evidence recommends, stated in this README).
- A legacy App without `status.activeRevision` reports the same `releaseGeneration` before and after the change (pinned by t003).
- Both shapes are covered by a store-reconciler test and an operator envtest that fail on the pre-fix code.

## Root cause

- `lego/operator/internal/controller/release_identity.go` `successfulReleaseGeneration` falls back to `status.image` + `observedGeneration` when `status.activeRevision` is empty, so a first release that never served can be recorded as the last successful one. The same fallback is the legacy path for Apps that predate `activeRevision`, which is why `w1/m160` t003 left it out.

## Source + Goal linkage

- **Source:** `w1/106` (found in `w1/m160` t003's blast-radius pass, 2026-09-15; parked 2026-09-16 pending store-reconciler evidence), promoted 2026-09-21 via `/pm-brainstorm for w1` item 4.
- **Goal linkage:** release-identity truth (`docs/ADR004-app-deployment.md`; `w1/m160`) — the deploy row bex-api closes from this field is what REST, GraphQL, MCP and the dashboard show as the deploy's outcome.
- **Expected outcome:** the last known path by which a never-served release can be reported as successful is closed; legacy Apps unaffected.
- **Why now:** `m160` shipped the sibling fix for the failure and cancel phases; this fallback is the remaining discrepancy, and the note itself says it needs a milestone's worth of evidence rather than a line change.
- **Render parity:** included, narrowly — deploy status on REST/GraphQL/MCP and the dashboard deploy page; Render closes a canceled deploy as `canceled` without advancing the live deploy.
