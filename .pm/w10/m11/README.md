# w10 · m11 — Billing charges honesty + blueprint sync diagnostics

**Worker:** worker10 **Goal:** the Billing and Blueprints dashboard surfaces stop asserting things that aren't true — the Charges card never flashes a contradictory claim about whether a number is an estimate or a real Stripe invoice total, a failed blueprint sync tells the user why it failed instead of a dead-end red badge, and every Blueprint Sync History row renders with consistent badge styling for the state the backend actually emits. **Status:** todo

## Tasks (in order)

| id   | title                                                                     | est | depends_on           |
| ---- | -------------------------------------------------------------------------- | --- | --------------------- |
| t001 | Charges card must not assert Estimate/Invoiced copy before data resolves   | 30m | —                     |
| t002 | Persist the real error when a blueprint sync fails                         | 40m | —                     |
| t003 | Expose sync error message via GraphQL, REST, and MCP                       | 30m | t002                  |
| t004 | Surface the sync error message in the dashboard's Sync History             | 35m | t003                  |
| t005 | Fix BlueprintStatusBadge's fallback for sync-run states (success/running)  | 25m | —                     |
| t006 | Render parity check (REST/GraphQL/MCP + dashboard)                         | 30m | t001, t003, t004, t005 |
| t007 | Simplify                                                                    | 20m | t006                  |
| t008 | Test coverage                                                               | 40m | t006                  |
| t009 | Closeout                                                                    | 10m | t008                  |

## Definition of done

- `/billing`'s Charges card never renders "An estimate, not an invoice." and then swaps to "The total is the amount Stripe will invoice." (or vice versa) for the same page load — while `invoicedUsd` is unresolved the card shows a neutral/loading state instead of committing to either claim, and it settles on exactly one, correct description once the query resolves.
- A blueprint sync that fails (`applyErr != nil` in `runSync`/`SyncBlueprint`) persists a human-readable error message on its `blueprint_syncs` row instead of discarding it.
- That message is readable via REST, GraphQL, and MCP on the sync-run shape, and the dashboard's Sync History surfaces it (tooltip, expandable row, or equivalent) so a user can learn why a sync failed without backend log access. Repro case: blueprint `blp-d9nqg95cavls73fp8m10` (discourse_docker) — its existing failed row obviously has no captured message (pre-fix data), but a fresh induced failure must show one end to end.
- Every state value `BlueprintSync.State` (`created`, `running`, `success`, `error`) and `Blueprint.Status` (`created`, `paused`, `in_sync`, `syncing`, `error`) renders through `BlueprintStatusBadge` with an intentional, styled variant and a proper label — none fall through to the raw-value `statusUnknown` fallback in normal operation. Repro: Sync History's "success" rows currently render as plain unstyled lowercase text next to a properly styled red "Error" badge.
- `yarn typecheck && yarn lint && yarn test` (dashboard) and `go test ./...` + `make lint` (backend, from `lego/operator/`) all green; new/changed migration applies cleanly.

## Source + Goal linkage

- **Source:** live Playwright QA hunt of `dashboard.bex.co` on 2026-08-21/22 (`/agents`, `/billing`, `/blueprints`, `/workspace/settings`, project + service detail pages). Cross-checked against the dashboard and backend source before filing: 5 of the original 8 `/agents`-surface findings (search box, stale "Recent" heading, Working/Hibernated status mismatch, idle-banner grammar, unbounded accessible-name text) turned out to be exact duplicates of `w9/done/m92`, itself seeded by this same QA session's `/agents` findings on 2026-08-21 — those are **not** re-filed here (see DO_NOT_DO's anti-duplication rule). The workspace-Team-members "internal id instead of email" observation was investigated and found to be intentional, documented fallback behavior (`dashboard/src/features/team/components/member-row.tsx`'s own doc comment: "falling back to their opaque userId when email is unresolvable — never blank") — not a bug, also not re-filed. This milestone covers the three findings that are genuinely new and still reproduce against current `main`: the Billing charges-copy flicker and the two Blueprint Sync History defects.
- **Goal linkage:** [docs/ADR040-billing-metronome.md](../../docs/ADR040-billing-metronome.md) — the Billing page is the trust surface for real money; it must never assert two different, conflicting claims about the same dollar figure. [docs/ADR049-render-yaml-parity.md](../../docs/ADR049-render-yaml-parity.md) — Blueprints are bex's `render.yaml` IaC surface; a sync that fails silently with no diagnostic path forces users (or support) to reach for backend logs, undermining the whole point of a self-serve dashboard.
- **Expected outcome:** Billing shows one honest, stable description of the Charges total on every load. A failed blueprint sync is self-diagnosable from the dashboard. Blueprint Sync History renders every real backend state value with consistent, intentional styling.
- **Why now:** all three are precisely root-caused already (exact file/line citations below, in hand from live QA cross-referenced against source) — small, well-scoped fixes rather than open-ended investigation. The billing item is a live correctness bug on a real-money page; the blueprint-diagnostics gap is a standing support-cost sink (`blp-d9nqg95cavls73fp8m10` has been sitting in an undiagnosable Error state for 12h+ as of the hunt).
- **Render parity task included** because t002–t004 add a new field to the `BlueprintSync` shape (REST/GraphQL/MCP all need it) and t005 touches a dashboard component shared between the `Blueprint.status` and `BlueprintSync.state` vocabularies — both need a REST/GraphQL/MCP/dashboard consistency pass. t001 is dashboard-only (no backend shape change) but rides the same check for completeness.
