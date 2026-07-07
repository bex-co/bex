# w5 · m3 — Internationalization (i18n), including Ory Elements

**Worker:** worker5 **Goal:** The dashboard renders in the user's language — both bex's own UI strings and the Ory Kratos auth flows follow a single selected locale, chosen via a switcher and persisted correctly across SSR and client hydration. **Status:** done

## Tasks (in order)

| id   | title                                                | est | depends_on             | status     |
| ---- | ----------------------------------------------------- | --- | ------------------------ | ---------- |
| t001 | i18n foundation: i18next core + provider              | 90m | —                        | — **DONE** |
| t002 | SSR-safe language detection + persistence             | 75m | w5/m3/t001               | — **DONE** |
| t003 | Language switcher in the dashboard chrome              | 45m | w5/m3/t001, w5/m3/t002   | — **DONE** |
| t004 | Extract app strings into feature locales               | 75m | w5/m3/t001               | — **DONE** |
| t005 | Localize Ory Elements auth flows to the locale         | 60m | w5/m3/t001, w5/m3/t002   | — **DONE** |
| t006 | Seed a second language (zh) + update docs/CLAUDE.md    | 45m | w5/m3/t004, w5/m3/t005   | — **DONE** |
| t007 | Simplify the i18n code this milestone added            | 30m | w5/m3/t006               | — **DONE** |
| t008 | Test coverage for i18n behavior                        | 45m | w5/m3/t006               | — **DONE** |

## Definition of done

- Switching the language in the dashboard changes **both** bex's own strings (sidebar, header, services page, auth hero copy, metrics, settings, 404/error) **and** the Ory-rendered Kratos flow forms (login, sign-up, forgot-password, settings) to the selected language.
- Language preference survives a full-page reload with **no React hydration mismatch** (verified: SSR renders the same language the client hydrates with).
- At least two languages ship end-to-end (`en` + `zh`); adding a third is a documented, mechanical step.
- Translation keys are namespaced per feature; a missing/unprefixed key is caught in dev.
- `dashboard/CLAUDE.md` no longer says "No i18n"; it documents the new convention.
- `yarn lint`, `yarn typecheck`, and `yarn test` pass.

## Source + Goal linkage

- **Source:** User request 2026-07-06 — "add a milestone in w5 to ensure i18n should work well; learn from `~/projects/web-beancount/beancount-dashboard`; ensure i18n works for Ory's components as well." Reference architecture: that project's `docs/i18n.md` (i18next + react-i18next, feature-based `locales/`, SSR→hydration language injection, dual cookie+localStorage persistence, namespaced keys, `useTranslations` hook with dev validation).
- **Goal linkage:** Advances the w5 dashboard pillar (bex's human-facing surface) — a Render alternative aimed at a global audience needs a localizable UI. Reverses the scaffold's deliberate "no i18n until there's a real requirement" stance (`dashboard/CLAUDE.md`), which this request now establishes.
- **Expected outcome:** A user can pick their language and the entire authenticated + auth experience — including the third-party Ory Elements forms, which don't automatically inherit an app's i18next instance — renders in it, persisted across reloads without hydration errors.
- **Why now:** The auth surface (w4) and dashboard shell (w5/m1–m2) are in place, so the string surface is stable enough to extract without churn. Ory Elements is a distinct integration risk (its flow components own their own copy and locale config, separate from react-i18next), so it's scoped as its own task rather than assumed to "just work" — doing it now, while the auth pages are few, is far cheaper than retrofitting later.
