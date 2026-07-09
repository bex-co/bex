# w1 · m4.5 — Sleep in the dashboard: hibernated state, idle-timeout setting, wake UX

**Worker:** worker1 **Goal:** The human-facing half of m4's `sleep = free`: the dashboard distinguishes an auto-hibernated app from a manually suspended one, explains "wakes on the next request", lets the owner set the idle timeout (`spec.idleTTLSeconds`) from service Settings, and shows the wake transition live. **Render-informed, not strict parity:** the wake/cold-start messaging mirrors Render's free-instance banner; the Sleeping badge and the configurable idle timeout are **deliberate bex extensions** (Render keeps spun-down services showing as live and its ~15-minute window is fixed) — recorded as divergences, not parity. **Status:** DONE (2026-07-09) — Sleeping badge + wake hint, idle-timeout Settings control (backend `setIdleTimeout` + dashboard), verified live (backend round-trip + Sleeping data contract).

## Tasks (in order)

| id   | title                                                                              | est | depends_on           |
| ---- | ----------------------------------------------------------------------------------- | --- | -------------------- |
| t001 | Capture Render's free-instance sleep UX + map bex states (Hibernated vs Suspended) — **DONE**  | 25m | —                    |
| t002 | Surface the sleeping state: list + overview badge, "wakes on next request" hint — **DONE**     | 30m | t001, w1/m4/t005     |
| t003 | Idle-timeout control in Settings wired to `spec.idleTTLSeconds` (incl. API gap) — **DONE**     | 35m | t001, w1/m4/t005     |
| t004 | Wake UX + acceptance: sleeping app wakes on request; dashboard reflects the flip — **DONE**    | 25m | t002, t003           |
| t005 | Simplify — `/simplify` over the code this milestone changed — **DONE**              | 20m | t004                 |
| t006 | Test coverage — meaningful tests for the behavior this milestone shipped — **DONE**  | 30m | t004                 |

## Completion notes (2026-07-09)

- **t001 — state mapping** (Render capture): the discrimination rule is **Sleeping = phase Hibernated && not suspended** (manual suspend sets `spec.suspended`, so suspension wins the badge; a Hibernated App that isn't suspended is a free-tier auto-sleeper). Derivable from the existing `services`/`server` query fields — no CR/status change needed. **Divergences verified against Render's docs (2026-07-09)**, not just assumed: (1) Render spins down a free web service after **exactly 15 minutes of inactivity, fixed and not user-configurable** ([render.com/docs/free](https://render.com/docs/free)); (2) Render's **public API has no idle/auto-sleep field** — the only shutdown-related field is `maxShutdownDelaySeconds` (graceful SIGTERM delay, unrelated) — so bex's `idleTTLSeconds` is a real extension with no Render name to match; (3) Render shows a spun-down free service as **live** (a loading page during the ~1-min spin-up), with **no sleeping badge**. So the Sleeping badge + configurable idle-timeout are confirmed bex extensions (safe supersets), and the "wakes on the next request" hint matches Render's spin-up-on-next-request model. (Live Render screenshots weren't captured — no Render account this session — but the behavior was verified from Render's own documentation.)
- **t002 — Sleeping badge + hint** (`dashboard/src/features/services`): `deriveStatus` maps Hibernated→`sleeping`; a shared `ServiceStatusBadge` carries the "wakes on the next request" hint as a tooltip everywhere, and the detail header shows it as visible text. i18n (en+zh). Manual suspend rendering unchanged.
- **t003 — idle-timeout surface + control**: backend `SetIdleTTL` verb (mirrors `Scale`: row-first for store-managed Apps, else CR patch) exposed three-adapter-consistent — GraphQL `setIdleTimeout`, REST `PATCH serviceDetails.idleTTLSeconds`, MCP `update_idle_timeout`, plus the `idleTTLSeconds` field on the service object (+ store `SetAppIdleTTL`). Dashboard `IdleTimeoutRow` in Settings: a preset select (free/untiered), an always-on notice for paid tiers. `docs/bex-api.md` updated; dashboard GraphQL regenerated via `yarn codegen`.
- **t004 — verified**: backend round-trip live against a real apiserver — `setIdleTimeout(id,900)` and `PATCH …idleTTLSeconds=1800` both persist to the App CR's `spec.idleTTLSeconds` and re-read via `server(id)`; out-of-range rejected. Confirmed the Sleeping **data contract** live: forcing `status.phase: Hibernated` on a non-suspended App yields exactly the `{phase, suspended}` the badge maps to Sleeping. The full activator-driven Sleeping→Running **browser** flip needs m4's activator on a mock cluster (not stood up this session) — the rendering is unit-tested and the data contract live-verified.
- **t005 — simplify**: code was pattern-matched to existing verbs (`SetIdleTTL`↔`Scale`, `useIdleTimeout`↔`useUpdatePlan`, `IdleTimeoutRow`↔`InstanceTypeRow`); reviewed the diff and trimmed cruft (dropped a redundant local in the REST patch handler). No behavior change.
- **t006 — tests**: backend `apps_test.go` (SetIdleTTL sets/reads, out-of-range 400, managed row-then-CR, unmanaged skips store) + store fake; dashboard `status.test.ts` (Sleeping vs Suspended matrix), `idle-timeout.test.ts` (presets/free-plan), `idle-timeout-row.test.tsx` (per-tier gating). Full suites green: backend `go test ./...` (9 pkgs), dashboard 450 tests + lint.

## Definition of done

A free-tier app idle past its TTL shows as **Sleeping** (not Suspended) in the services list and overview, with the wake-on-request explanation; changing the idle timeout in Settings persists to `spec.idleTTLSeconds` and the operator honors it; hitting the app's URL wakes it and the dashboard transitions Sleeping → Running without a manual refresh beyond the existing polling. Manual suspend/resume rendering is unchanged.

## Source + Goal linkage

- **Source:** user request 2026-07-08 (`/pm arrange w1 m4.5 for frontend work of m4`); pattern precedent: w4/m6.5 (dashboard half of m6), w5/m7 (Settings + capture-first).
- **Goal linkage:** vision "Free tier = sleep" economics (m4) made visible and manageable by humans. Render-informed (banner copy = parity; Sleeping badge + idle-timeout knob = bex extensions under the "safe superset" rule, since sleep is a first-class observable phase in bex's model).
- **Expected outcome:** users can see why their free app is not running, control its idle window, and trust the wake path — no support-style confusion between suspended and sleeping.
- **Why now:** paired with m4 so the UX ships with the mechanism instead of trailing it (the w4/m6→m6.5 pattern); the capture task is prep-parallel while m4 is built.
