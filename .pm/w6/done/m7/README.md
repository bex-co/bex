# w6 · m7 — User owners: opaque `own-` identity ids for member identifiers

**Worker:** worker6 **Goal:** stop leaking raw Kratos subjects as user identifiers on the owners surface: mint a stable, opaque `own-<xid>` id per identity and report each workspace member's `userId` as their `own-` id on the Render owners surface — the one genuinely-Render-faithful gap in the `owners` API. **Status:** done — 2026-07-11 (id.Owner + `owner_ids` table/migration 0009 + `OwnerIDForSubject`; members `userId` now an own- id, verified through the full REST stack + a real-Postgres round-trip; owner `type`/peer-resolve documented as non-goals; lint + full backend suite green)

> **Scope note (trimmed 2026-07-11 after a design review, refined during implementation).** An earlier draft also planned owner `type: user` and _peer_ `own-`→workspace resolution; both were dropped as **not real parity for bex** (only `tea-` team workspaces exist, so `type` stays `"team"`; peer-resolve has no consumer) — recorded as non-goals in t003, not built. Implementation further found the own- `userId` belongs **only** on the Render REST `/v1/owners/{id}/members` surface: the `internal/members` package (w4/m12's dashboard GraphQL/MCP team management) uses a `subject`-named field and takes `subject` as _mutation input_, so it deliberately stays subject-keyed (mapping it would break the mutation round-trip and needs no `own-` reverse lookup). So the real change is one field on one REST endpoint, backed by a real per-identity id.

## Tasks (in order)

| id   | title                                                                                          | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | `own-` id kind + subject⇄own-id persistence (id registry, store table + migration, resolver)   | 45m | — — **DONE**    |
| t002 | Members `userId` = the member's `own-` id on the Render owners surface (the real parity fix)    | 30m | t001 — **DONE** |
| t003 | Render parity — `own-` identifiers consistent; record the deliberate non-goals                  | 20m | t002 — **DONE** |
| t004 | Simplify — `/simplify` over the code this milestone changed                                      | 15m | t003 — **DONE** |
| t005 | Test coverage — meaningful tests for the behavior this milestone shipped                         | 25m | t003 — **DONE** |
| t006 | Closeout — move milestone to done when DoD holds                                                 | 10m | t005 — **DONE** |

## Definition of done

On a cluster with `BEX_CP_DB_URI`: every identity resolves to a stable `own-<xid>` id (same id across calls; minted through `internal/id` as `id.Owner`, persisted subject⇄own-id in `owner_ids`). `GET /v1/owners/{id}/members` reports each member's `userId` as their `own-` id, not a raw Kratos subject, and the mapping is stable across calls. Owner objects keep `type: "team"` (bex has no user-owner entity — documented, not a bug). The `internal/members` dashboard surface stays `subject`-keyed (documented — it's a bex-native contract with subject-keyed mutations, not the Render `userId` surface). `docs/render-artifacts/owners-api.md` and `w6/done/001.md` are updated: member-identifier parity resolved; owner `type: user` and peer `own-` resolution recorded as deliberate non-goals with reasons. No new REST mutation (owners stays read-only).

## Source + Goal linkage

- **Source:** user request 2026-07-11 (`/pm how to implement own- or owner feature`) + the design review that followed, tracing `w6/done/001.md` item 1 to its root: bex emits no `own-` ids (`renderOwnerType = "team"` hardcoded; member `userId` = raw subject — `lego/backend/internal/workspaces/render.go`).
- **Goal linkage:** `docs/ADR008-vision.md` pillar 1 (Render parity). Closes the member-identifier cell of the `owners` read-API vs Render's OpenAPI; explicitly records the two adjacent behaviors that are non-goals for bex's all-teams model.
- **Expected outcome:** the owners/members surface emits opaque, Render-shaped `own-` user ids instead of leaking internal Kratos subject strings — a small fidelity + information-hygiene win.
- **Why now (honest — low-priority polish):** this is **not** a blocker and has no hard why-now. The only sequence rationale is soft: `w4/m12` (members) renders user identifiers, so landing opaque `own-` ids first avoids retrofitting raw subjects into that UI/API later. If `w4/m12` isn't imminent this is a fair **defer** — flagged so the choice is explicit. Leaking a Kratos subject as `userId` is a fidelity/hygiene issue, not a correctness bug.
- **Render parity task included:** yes — changes the REST/GraphQL/MCP member shape, so t003 checks all three and records the non-goals. (Dashboard needs no change — it keys users by Kratos session, not `own-` id.)
