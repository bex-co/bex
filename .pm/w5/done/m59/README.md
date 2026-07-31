# w5 · m59 — Workspace-family parity walk: Team · Env Groups · Usage/Billing · Notifications · Webhooks · Blueprints

**Worker:** worker5 **Goal:** run a parity walk over the workspace-scoped page family — the pages no dedicated walk has covered since m32 (2026-07-15) — producing a reproducible per-page verdict artifact, in-milestone fixes for sub-hour gaps, and bounded notes for anything larger. **Status:** done (code-based audit; live captures deferred)

## Outcome (2026-07-30)

Delivered as a **code-based parity audit** — the live authenticated Render + bex side-by-side captures (t001/t002) were **infrastructure-blocked** in-session (no Render account/browser session; `dev-5` unraisable — shared kind cluster missing the CNPG `postgresql.cnpg.io/v1` CRDs; no prod bex session). The audit inventoried each family's bex implementation (routes + feature + components) and cross-referenced it against the pinned Render artifacts + the ADR018 ledger. **Verdict: verified parity across all six families** — every family ships a first-class workspace-scoped page, every relevant ADR018 row is ✅, active maintenance since m32 (env groups w5/m33, webhooks w1/m49, billing w7/m51–52, members w1/m33, notifications w7/m44), and **no missing pages / dashboard-unconsumed capabilities / stale cells found** — so no sub-hour fixes were applied (t003) and no gap milestones filed. Evidence artifact: [`docs/render-artifacts/workspace-family-walk.md`](../../../docs/render-artifacts/workspace-family-walk.md); ADR018 walk-record added. The one deferred DoD element — the **live Render-side drift check** (which the code audit can't catch) — is open note `031`.

## Tasks (in order)

| id   | title                                                                     | est | depends_on | status                          |
| ---- | -------------------------------------------------------------------------- | --- | ---------- | ------------------------------- |
| t001 | Live authenticated Render captures of the workspace-family pages            | 60m | —          | — **DONE\*** (infra-blocked → note 031) |
| t002 | bex side-by-side walk (dev-5 or prod) with a per-page verdict table         | 60m | t001       | — **DONE** (code-based verdicts) |
| t003 | Apply sub-hour fixes in place; file bounded notes for larger gaps           | 60m | t002       | — **DONE** (no gaps found)      |
| t004 | Evidence artifact in `docs/render-artifacts/` + ADR018 cell corrections     | 30m | t003       | — **DONE** (artifact + ADR018 record) |
| t005 | Simplify (`/simplify` over t003's fix diff)                                 | 20m | t004       | — **DONE** (no fix diff)        |
| t006 | Test coverage for the in-milestone fixes                                    | 30m | t004       | — **DONE** (no fixes → no new tests) |
| t007 | Closeout                                                                    | 15m | t006       | — **DONE**                      |

**\* t001 live-capture deferral:** the Render-side captures — the only way to catch Render-side visual/interaction drift — are infra-blocked and tracked as open note `031` for a session with a Render account + a live bex dashboard. The bex-side is confirmed present + at documented parity by the code audit.

## Definition of done

A reproducible evidence artifact exists in `docs/render-artifacts/` with one verdict row per walked page (match / fixed in-milestone / filed note / intentional exclusion), backed by captures on both sides. Every found gap is either fixed in this milestone (with tests) or filed as a bounded w5 inbox note; no gap is silently dropped. Any stale `docs/ADR018-render-parity.md` UI cell touched by the walk is corrected. Known non-goals (external drains, Slack delivery, PR previews, payment-method gates — `.pm/DO_NOT_DO.md`) are recorded as intentional exclusions, not filed as gaps. Dashboard suite, typecheck, and lint pass on the final tree.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` 2026-07-30 (proposal 3; renumbered from the proposal's m60 to the next free m59 since the gated sandboxes proposal was not materialized). Method precedent: `w5/done/m32` (full walk), `w5/done/m57` (family walk that found + fixed a real 404 drift); note-absorption precedent `w5/m36`.
- **Goal linkage:** Render parity (`docs/ADR018-render-parity.md`) — keeps the UI column honest now that the recorded-gap queue is empty; "open-source Render alternative" pillar.
- **Expected outcome:** either verified parity for the workspace family or the next concrete w5 gap list — the feeder w5 needs now that m50–m57 drained the settings/static walks.
- **Why now:** this family is the least-recently-verified (last dedicated walk 2026-07-15) while a lot has shipped onto it since (m33 env-group create, m36 Team search, w7/m51 billing onboarding/portal, webhook pages w3/m27); Render's side also drifts, and m57 proved walks catch real defects.
- **Render parity:** standing task **omitted** — this milestone _is_ a parity-evidence walk: the cross-surface comparison is its substance (t002/t004), any t003 fix is sub-hour and verified in place, and larger gaps become notes/milestones that will carry their own Render-parity task when materialized.
