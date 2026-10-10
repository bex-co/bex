# w4 · m193 — Keep Key Value logical storage separate from physical PVC capacity

**Worker:** worker4 **Goal:** a new default Free store’s disk capacity reports its 1 GiB logical allocation, retaining real grow-only allocations without promoting the provider’s physical floor. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Preserve logical Key Value allocation through storage reconciliation | 65m | — |
| [t002](t002.md) | Audit shared storage callers and the prior capacity guarantees | 35m | t001 |
| [t003](t003.md) | Render parity and API/dashboard acceptance | 20m | t001, t002 |
| [t004](t004.md) | Simplify | 15m | t003 |
| [t005](t005.md) | Test coverage | 35m | t003, t004 |
| [t006](t006.md) | Closeout | 15m | t005 |

## Definition of done

- On a newly created default **Free** Key Value with no storage override, wait for Available, open Metrics, and reload: **Capacity 1 GiB**, matching the catalog’s `storageGB: 1`, even where its physical volume is 10 GiB.
- Repeat the exact REST/GraphQL/MCP probes in [finding.md](finding.md) on that fixture: capacity is **1073741824 bytes** on all three surfaces; the UI uses that value. Neither a clamped dashboard label nor renaming the physical value satisfies this.
- On that fixture set a benign expiring marker/counter, suspend/resume and rename: values and absolute expiry survive, endpoint/id remain stable, and logical capacity remains 1 GiB. Those lifecycle/data controls passed in the source hunt and must continue passing.
- The read-only 1 GB Postgres control still reports **1073741824 bytes / 1 GiB**, with unchanged Details storage. Physical ops alerts and disk-used series retain their contracts.
- The shared-caller audit, growth/shrink/resize regression tests and explicitly recorded unverified sibling disposition are complete before closeout. Delete owned fixtures, revoke only the verification session and record actual acceptance evidence.

## Source + Goal linkage

- **Source:** user-requested live `$qa-find-bugs` loop, 2026-10-10 pass a36, muse.env; [finding.md](finding.md) contains complete creation, catalog and three-surface capacity probes. Owned Free store `red-db4u78bvjfbs73b08h40` was deleted and its session revoked.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) reliable hosting/observability; [ADR021](../../../docs/ADR021-keyvalue-management.md) Free 1 GB persistence; [ADR010](../../../docs/ADR010-observability.md) metrics; [ADR006](../../../docs/ADR006-bex-api.md) consistent APIs. This closes the live Key Value residual in `w4/done/m91`.
- **Expected outcome:** logical storage is derived from accepted tenant intent and real growth, while physical capacity remains available for resize readiness and operations. Users see the catalog-backed capacity rather than an unsolicited provider-floor allocation.
- **Why now:** the completed metrics fix already reads logical status, but the operator feeds that status from physical capacity. Reusing the old metrics-only fix cannot correct the live result.
- **Sizing:** 185m across two substantive implementation/audit tasks and the standing closing tasks; this requires preserving grow-only behavior and checking consumers, beyond a label edit.
- **Render parity:** included because the allocation reaches REST, GraphQL, MCP and UI. Render Free Key Value has no persistence; bex’s durable 1 GB Free disk is deliberate. Do not remove persistence to imitate Render.
- **Unverified:** live CR/PVC fields, service-disk floor behavior, a 5 GB Postgres control, larger KV growth/plan changes and billing impact. These are carried in the audit task; the source hunt makes no charge or quota claim.
