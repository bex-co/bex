# w22 · m313 — Sandbox storage and memory meters: a suspended sandbox pays snapshot storage, as on Render

**Worker:** worker22 **Goal:** a suspended sandbox is billed for its retained image and nothing else, and memory is metered on its own, as the first half of Render's sandbox billing model **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Design: where snapshot bytes come from, and the storage meter's event contract | 1h | — |
| t002 | Memory meter: record sandbox_memory_gb_seconds beside the existing meter, not yet billed | 1h | t001 |
| t003 | Storage meter: store, suspend-path hooks, and a registration API for snapshots | 1h30m | t001 |
| t004 | Storage bounds and garbage collection (ADR104 D11) | 1h30m | t003 |
| t005 | Rates, usage surfaces and Stripe line for storage; memory shown but unbilled | 1h30m | t002, t003 |
| t006 | Production check: suspend, accrue, resume, compare with the registry | 1h | t005 |
| t007 | Amend ADR023, ADR030 and the ledgers | 30m | t006 |
| t008 | Render parity | 30m | t007 |
| t009 | Simplify | 20m | t008 |
| t010 | Test coverage | 40m | t008 |
| t011 | Closeout | 10m | t009, t010 |

## Definition of done

- A suspended sandbox accrues `sandbox_snapshot_gb_seconds` from commit success until its image is deleted, idempotently; the metered bytes match the registry within the documented tolerance on production
- Snapshot storage is priced at Render × 0.70 and billed; the memory kind is recorded and visible but not billed; `sandbox_compute_seconds` and the agent-session cost are unchanged
- Per-workspace storage cap, image garbage collection and a registry-growth alert exist
- ADR023, ADR030 and ADR104 D8 are amended; w32/m285 has a documented API to register snapshot objects

## Source + Goal linkage

- **Source:** [docs/ADR104-sandbox-render-parity.md](../../../docs/ADR104-sandbox-render-parity.md), whose gap table is the proposal (no separate `/pm-brainstorm` run), filed by `/pmx adr104 to w22` on 2026-11-10; gaps 8 (storage and memory meters) and 14 (storage part)
- **Goal linkage:** Render parity first (ADR008): Render's suspended sandboxes pay snapshot storage only; usage metering and billing (ADR023, ADR030, ADR040); pillar 5 sandboxes
- **Expected outcome:** A suspended sandbox shows a storage charge and no compute charge; the usage surfaces list the new kinds; registry growth from sandboxes is capped and alerted
- **Why now:** Today a suspended sandbox is free and bex absorbs its retained image for up to 30 days, and w32/m285 would add 30-day objects of up to 40 GB at no cost to the creator (ADR104 D8). It has no technical dependency, so it ships first.
- **Render parity:** included, usage surfaces change.
- **Recommendation, not a user decision:** ADR104 D8 recommends adopting Render's model; the parity rule makes it the default, so these tasks implement it. The user's sign-off is only needed if w22/m314's CPU spike fails.
- **Rates:** Render × 0.70 per the `pricing.yaml` convention; Render's $0.08 per GB-month comes from render.com/articles/render-sandboxes-vs-vercel-sandboxes (2026-11-06), not from Render's pricing page. Re-read it before shipping.
- **Shadow memory meter:** recorded here, billed by w22/m314, because the existing weighted charge cannot be split.
