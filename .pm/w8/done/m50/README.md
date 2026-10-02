# w8 · m50 — Enforce Postgres tier capacity across write paths

**Worker:** worker8 **Goal:** free Postgres stays within its tier, and supported paid configurations remain usable. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Shared Postgres admission and server-owned capability metadata — **DONE** | 45m | — |
| t002 | Blueprint validation and merged-state admission — **DONE** | 35m | t001 |
| t003 | Dashboard storage and capability controls — **DONE** | 40m | t001 |
| t004 | Stop unsupported automatic storage growth without shrinking data — **DONE** | 20m | t001 |
| t009 | Provision eligible readers and account for their full capacity — **DONE** | 45m | t001, t004 |
| t005 | Render parity — **DONE** | 20m | t002, t003, t004, t009 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage and local acceptance — **DONE** | 35m | t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

## Definition of done

- Free create/update requests cannot request storage above 1 GB, enable autoscaling or pooling, or add read replicas; each gets an actionable named refusal before writes. All direct adapters and dry runs share admission.
- Replicas require at least 0.5 CPU and 10 GB effective storage, with at most five. The existing `basic-1gb` plan at 10 GB supplies a supported control; HA retains its separate 1-CPU rule. No plan or price is added.
- Plan changes cannot strand existing replica, pooler, autoscaling, or allocated-storage state on an unsupported tier. Existing data is never shrunk or removed; unrelated edits and disabling unsupported features remain possible.
- Blueprint validation locates refusals, and plan/apply checks the merged existing resource before mutation, preserving omitted fields and allowing explicit replica removal.
- Dashboard controls consume server capability metadata and display useful refusals. Unsupported autoscaling cannot be enabled; existing unsupported settings can still be turned off.
- Operator autoscaling does not grow free databases, including pre-existing flagged ones, and preserves paid growth. Full envtest verifies this backstop; the shared development operator is not replaced for acceptance.
- Eligible named readers produce actual CNPG replicas independently of HA. Desired instance count, readiness and autoscale quota account for all instances; unsupported legacy declarations do not newly add capacity or silently remove running instances. Named endpoints continue to share the CNPG reader pool.
- Relevant suites, lint, review, and a compact authenticated local API/CLI acceptance matrix pass; owned tenant fixtures are deleted and evidence limitations are recorded.

## Source + Goal linkage

- **Source:** [035 production findings](../035.md), promoted 2026-10-02; related [m43 HA decision](../m43/README.md).
- **Goal linkage:** managed Postgres correctness and Render-compatible admission (ADR006, ADR009, ADR018), with the free-tier capacity rule in ADR030.
- **Expected outcome:** sibling flags cannot turn a free database into paid-tier capacity; valid paid configurations and data-preserving transitions remain available.
- **Why now:** production accepted replicas and 15 GB autoscaling storage on free. Fixing only HA left the same capacity escape through sibling settings.
- **Render parity included:** REST, GraphQL, MCP, Blueprint and dashboard behavior change. Primary sources checked 2026-10-02: [replicas](https://render.com/docs/postgresql-read-replicas) require 0.5 CPU, 10 GB, maximum five; [free storage](https://render.com/docs/free) is fixed at 1 GB; [pooling](https://render.com/docs/postgresql-connection-pooling) requires a paid plan. This corrects the note's provisional 1-CPU replica assumption.

## Scope

Preserve the current catalog and prices. No production migration, database shrink, tenant-resource deletion, new replica-management UI, or shared local operator replacement. Existing unsupported configurations are handled without destructive conversion; policy-specific updates are checked against the resulting state.

## Acceptance finding — 2026-10-02

The paid positive control exposed an unfinished part of w1/m22: named read replicas were only aliases to the CNPG `-ro` service. The operator still requested one instance with HA off, so the live endpoint had no ready readers. Task t009 closes that mechanism gap within this milestone so the original replica control can work. The 59 passing API/CLI admission controls remain separate from runtime proof. The shared development operator remains unchanged; envtest proves reconciliation, and a separately named CNPG-only fixture verified the exact generated projection without a competing manager.

## Closeout — 2026-10-02

All nine tasks are done. [Acceptance](acceptance.md) records shared admission, server-driven UI, working paid readers, complete automated verification, 59/59 live admission controls, runtime replication and cleanup. The shared operator and production were not changed during verification; the exact CNPG projection was exercised independently. Base dev-8 and test services remain available.
