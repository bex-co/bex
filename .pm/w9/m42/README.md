# w9 · m42 — Datastore metadata parity on GraphQL/MCP: region · dashboardUrl · updatedAt

**Worker:** worker9 **Goal:** the Render resource metadata REST ships for Postgres and Key Value (`region`, `dashboardUrl`, authoritative `updatedAt`) exists with identical semantics on their GraphQL and MCP surfaces — closing the divergence w9/m41's audit documented and locking it with cross-surface tests — and the dashboard (GraphQL-only) can finally show a datastore's region, as Render's does. **Status:** todo

## Tasks (in order)

| id   | title                                                                                    | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Add `region`/`dashboardUrl`/`updatedAt` to Postgres + KeyValue GraphQL types via `resourcemeta` | 45m | —          |
| t002 | Mirror the fields on the MCP get/list tool outputs                                         | 30m | t001       |
| t003 | Extend m41's cross-sibling parity tests to lock the new fields; update the ADR018 note      | 30m | t002       |
| t004 | Dashboard: render region on database/keyvalue detail pages                                  | 30m | t003       |
| t005 | Render parity — cross-surface field/semantics check vs Render's datastore objects           | 20m | t004       |
| t006 | Simplify — `/simplify` over the milestone's diff                                            | 20m | t005       |
| t007 | Test coverage — meaningful tests for the shipped behavior                                   | 20m | t005       |
| t008 | Closeout — move to `done/` when the DoD holds                                               | 15m | t007       |

## Definition of done

The same Postgres or Key Value store fetched over REST, GraphQL, and MCP returns identical `region`, `dashboardUrl`, and `updatedAt` values (each omitted consistently when unconfigured, per the `BEX_REGION`/`BEX_DASHBOARD_URL` gating); a cross-sibling parity test fails if any surface drops one; the dashboard datastore detail pages show region when configured.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 21, 2026-07-15 — closeout-residual sweep over `w9/done/m41/done/t003.md` ("GraphQL additionally drops `updatedAt` entirely and Postgres/KeyValue GraphQL/MCP carry no `region`/`dashboardUrl` at all … recorded as a follow-up in ADR018, not built") + dashboard-gap mine G4 (the dashboard is GraphQL-only, so it structurally cannot show datastore region — verified: `postgres/rest.go:44-45` has the fields, both datastore `graphql.go` files have zero hits).
- **Goal linkage:** Render compatibility — the three-adapter rule (`docs/ADR006-bex-api.md`); closes an ADR018-documented divergence with test-locked evidence.
- **Expected outcome:** datastore metadata is surface-symmetric; the dashboard gains region display (Render shows it on datastore detail).
- **Why now:** m41 closed yesterday having built the resolver seam (`resourcemeta`) and the parity-test harness this milestone extends — only wiring remains; w9's queue is one rollout milestone.
- **Render parity:** included (t005) — feature dev touching GraphQL/MCP/UI.
