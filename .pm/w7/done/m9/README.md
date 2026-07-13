# w7 · m9 — Per-workspace abuse limits (creation caps + build concurrency)

**Worker:** worker7 **Goal:** One workspace cannot exhaust the shared tenant namespace: service/datastore creation is capped per workspace at the API layer (Render-Hobby-shaped defaults), and concurrent builds are bounded at the operator layer (Render's one-active-build-per-service, newest-wins) — all env-tunable, byte-identical when unset. **Status:** done

## Tasks (in order)

| id   | title                                                                                             | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Design the limit set: caps, env knobs + Render-anchored defaults, error shape; document in ADR006 | 30m | —          | — **DONE** |
| t002 | bex-api: enforce creation caps in the core create verbs (one check, all three surfaces)           | 45m | t001       | — **DONE** |
| t003 | Operator: newest-wins build serialization per App + per-workspace concurrent-build cap            | 45m | t001       | — **DONE** |
| t004 | Render parity — same caps/error shapes across REST/GraphQL/MCP + dashboard; compare Render        | 30m | t002, t003 | — **DONE** |
| t005 | Simplify — `/simplify` over the code this milestone changed                                       | 20m | t004       | — **DONE** |
| t006 | Test coverage — meaningful tests for cap enforcement + build serialization                        | 30m | t004       | — **DONE** |
| t007 | Closeout — DoD verified, milestone moved to `done/`                                               | 15m | t006       | — **DONE** |

## Definition of done

With caps configured: the (N+1)th service create for a workspace is refused with the documented error shape on REST, GraphQL, and MCP while a second workspace can still create (same for databases and key-value instances at their caps); a push-spam burst for one App yields at most one active build Job for that App (newest wins, superseded build cancelled) and no more than the configured per-workspace concurrent total — observable via `kubectl get jobs`. With every knob unset, behavior is byte-identical to today. The limits and their Render anchors are documented in `docs/ADR006-bex-api.md`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w7` round 2 (2026-07-12). Verified that day: `deploy/gitops/base/tenant-quotas.yaml:19-24` is aggregate across all tenants in the shared apps namespace (100 vCPU / 200 GiB / 1000 pods / 500 Jobs — a cluster backstop, not a fairness boundary); `lego/backend/internal/core/` has no create-path count guard; `lego/operator/internal/build/build.go` has per-Job hygiene but nothing serializes or caps concurrent builds, and the git webhook is rate-limit-exempt (m3) so build spam is free.
- **Goal linkage:** GOAL.md V0 #5 (multi-tenant) and #7 (security review) — completes the w7 abuse-hardening pair: m3 bounded the request layer, m9 bounds the resource layer above m2's collective backstop.
- **Expected outcome:** noisy-neighbor DoS via resource creation or build spam is closed per workspace; the shared quota returns to being a backstop rather than the only line.
- **Why now:** verified that nothing bounds a single workspace; cheapest to land before real tenants exist to calibrate against.
- **Render parity: included** (t004) — this touches create verbs and error shapes on REST/GraphQL/MCP plus dashboard error display. Anchors verified against render.com docs 2026-07-12: Hobby plan = "limited to 25 total services" per workspace; free tier = one active free Postgres + one active free Key Value per workspace; builds = "Each Render service can have only one active build at a time … Render cancels any in-progress build for the same service". **Conscious divergences to record:** the per-workspace *concurrent-build* cap has no Render equivalent (Render bounds build abuse via billed pipeline minutes; bex has no billing, so a hard cap substitutes), and Render publishes no API error shape for its service cap (dashboard-blocked), so bex picks its own consistent shape — the m6 pattern.
- **Architecture check:** creation caps live in bex-api (business logic, ADR003); build serialization lives in the operator (mechanism). No boundary violation.
