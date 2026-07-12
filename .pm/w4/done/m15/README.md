# w4 · m15 — Settings → Security & Compliance grouping (move Audit Log)

**Worker:** worker4 **Goal:** Move the in-app Audit Log card out of the flat `/settings` column into a dedicated **Security & Compliance** section grouping, aligning bex's Settings IA with Render's Workspace Settings → Compliance placement and creating the home the planned session-management (w4/006) and MFA (m11) cards also belong in. **Status:** done

## Tasks (in order)

| id   | title                                                              | est | depends_on  | status        |
| ---- | ------------------------------------------------------------------ | --- | ----------- | ------------- |
| t001 | Introduce a Security & Compliance section grouping on Settings     | 40m | —           | — **DONE**    |
| t002 | Move AuditLogPanel into the Security & Compliance section          | 20m | w4/m15/t001 | — **DONE**    |
| t003 | Render parity: Settings IA vs Render Workspace Settings→Compliance | 20m | w4/m15/t002 | — **DONE**    |
| t004 | Simplify the settings-page changes                                 | 15m | w4/m15/t003 | — **DONE**    |
| t005 | Test coverage: Audit Log renders under the new section             | 30m | w4/m15/t003 | — **DONE**    |
| t006 | Closeout                                                           | 5m  | w4/m15/t005 | — **DONE**    |

## Definition of done

On `/settings`, the Audit Log card no longer sits as a bare sibling of Team/API Keys in the flat column: it renders inside a labelled **Security & Compliance** section (section heading + grouped card container) that is structured to also host the future session-management (w4/006) and MFA (m11) cards. A vitest test on the settings page asserts the Audit Log card renders under that section heading (not as a top-level sibling), and the admin-only gating + store-less 503 states of `AuditLogPanel` are unchanged. `yarn lint` + `yarn test` green. The inbox note `w4/007` is closed (moved to `w4/done/007.md`).

## Source + Goal linkage

- **Source:** inbox note `w4/007` (IA-placement drift found in m14 t007 render-parity pass, 2026-07-11) — user resolved the flagged product decision as "yes, move it" (2026-07-12), which per 007's own promotion condition materializes as this milestone.
- **Goal linkage:** w4 (auth & identity / multi-tenant-secure) — Settings IA coherence for the security/compliance surfaces (audit log, sessions, MFA); parity ledger `docs/ADR018-render-parity.md` (Render's audit-log lives under Workspace Settings → Compliance).
- **Expected outcome:** Audit Log is discoverable under a Security & Compliance grouping that matches operator expectations from Render and gives w4/006 + m11 a coherent home, instead of an accidental placement beside Team/API Keys premised on a mistaken read of Render's IA.
- **Why now:** The mistaken premise is already documented (ADR018) and the fix is cheap to do before w4/006 (session mgmt) and m11 (MFA) each independently add cards to the same surface — doing the grouping once now avoids a second Settings restructure and forced re-placement later. **Render parity task included:** this is a dashboard UI-surface change, so the parity check (Settings IA vs Render's actual Compliance placement) applies.
