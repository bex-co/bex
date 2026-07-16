# w4 · m23 — Audit & payload-parity chores round 2

**Worker:** worker4 **Goal:** Close the four open w4 inbox notes in one grouped pass (the m20/m23-style chores pattern): audit-log `direction` honored, `suspenders` on the service object, the denied-read audit fan-out latency fix, and Render's Environment `ipAllowList` wire shape. **Status:** done 2026-07-15 — t001–t005 + t007 implemented by a parallel session working the inbox notes directly (resolutions in each task file + `../013`–`017`); t006's simplify pass applied five cleanups over `cc2156b8` (shared `core.ParseDirection`, one `pageKeyset` helper, `auditsDenial` predicate, zero-I/O aggregate-denial workspace, no redundant PATCH `Get`) and filed the declined-depth findings as `w4/022.md`

## Tasks (in order)

| id   | title                                                                                                       | est | depends_on             |
| ---- | ------------------------------------------------------------------------------------------------------------ | --- | ---------------------- |
| t001 | Honor the audit-log `direction` param or 400 it (REST + GraphQL) — **DONE**                                              | 30m | —                      |
| t002 | `suspenders` array on the service object (`["user"]` on user-suspend, `[]` otherwise; never faked values) — **DONE**      | 30m | —                      |
| t003 | Denied-read audit fan-out: decide detach-vs-cap for `AuthorizeApp`'s name-collision loop, implement, regression-test — **DONE** | 60m | —                      |
| t004 | Environment `ipAllowList` REST wire shape: `{cidrBlock, description}` objects on standard POST/PATCH — **DONE**           | 30m | —                      |
| t005 | Render parity — cross-surface consistency check for t001/t002/t004; compare Render behavior, flag drift — **DONE**        | 20m | t001, t002, t003, t004 |
| t006 | Simplify — `/simplify` over the code this milestone changed — **DONE**                                        | 20m | t005                   |
| t007 | Test coverage — meaningful tests for each chore's behavior + failure modes — **DONE**                                     | 30m | t005                   |
| t008 | Closeout — DoD met → move milestone to `done/` — **DONE**                                                     | 10m | t007                   |

## Definition of done

`direction=asc` returns oldest-first (or a named 400 — nothing accepted is ignored) on REST + GraphQL; a user-suspended service carries `suspenders: ["user"]` (and `[]` when running) across REST/GraphQL/MCP; a denied read against a many-way colliding service name no longer serializes N×2s-bounded synchronous audit writes before its 403, with the chosen semantics (detach vs. cap) recorded; `PATCH`/`POST` on Environments accept and return `ipAllowList` as `{cidrBlock, description}` objects and the conformance suite passes the route without an allowlist entry.

## Source + Goal linkage

- **Source:** groups the four open w4 notes — `013` (direction param, round-3 census), `014` (suspenders, round-7 field-diff), `015` (audit fan-out, filed by `/simplify` over m20's diff), `017` (Environment ipAllowList wire drift, filed by m22's Render-parity closeout). Proposed in `/pm-brainstorm` rounds 8–9.
- **Goal linkage:** Render payload parity (pillar 1) + w4's multi-tenant-security mandate (audit correctness and latency on the read-dominated path).
- **Expected outcome:** three documented ADR018/conformance divergences close; a real latency/DB-load risk on cross-tenant name collisions is fixed.
- **Why now:** w4's queue is empty; all four notes were filed by recent closeouts and t003 degrades real request latency today.
- **Render parity closing task: included** — t001/t002/t004 touch REST/GraphQL/MCP shapes.
