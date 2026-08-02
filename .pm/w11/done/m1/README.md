# w11 · m1 — Seed and sanitize the Expo mobile foundation

**Worker:** worker11 **Goal:** copy the mature Beancount Expo app into `mobile/`, remove every accounting/product/release identity, and retain a compiling, attributed bex-neutral shell with the reusable layout, utilities, controls, and charts needed by later milestones. **Status:** done (2026-08-02; curated source snapshot at `21cbbc9…`, provenance/licenses retained, product and credential identity removed, 36 unit tests + Expo Doctor 20/20 + iOS/Android bundle exports green; no simulator or Android SDK was available for an interactive launch)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Import the source snapshot with provenance and license inventory — **DONE** | 30m | — |
| t002 | Strip Beancount product, repository, release, and credential identity — **DONE** | 45m | t001 |
| t003 | Retain and neutralize reusable layout, utilities, components, and charts — **DONE** | 60m | t002 |
| t004 | Rebrand Expo configuration and add the minimal bex shell — **DONE** | 45m | t003 |
| t005 | Add mobile documentation, dependency audit, and CI gates — **DONE** | 45m | t004 |
| t006 | Simplify — **DONE** | 20m | t005 |
| t007 | Test coverage — **DONE** | 45m | t005 |
| t008 | Closeout — **DONE** | 10m | t007 |

## Definition of done

`mobile/` records source commit `21cbbc9ba6c058876bd7dc8636452531c2ba3b79` and preserves its MIT notice plus JetBrains Mono OFL; the root Apache license is unchanged. No inherited secret, Expo project id, bundle id, store metadata, analytics identity, generated Beancount schema, accounting route, or editor/camera permission remains. `rg -ni 'beancount|ledger|transaction|receipt|account' mobile` is clean except attribution, and the bex shell launches on iOS and Android with typecheck, lint, unit tests, formatting, and Expo configuration checks green.

## Source + Goal linkage

- **Source:** user request 2026-08-02; `docs/ADR048-mobile.md`; read-only agent audit of `/Users/tianpan/projects/beancount-io/mobile` at `21cbbc9…`.
- **Goal linkage:** ADR008's open-source product goal and ADR048's mobile mission; reuses a proven Expo foundation without importing another product's domain.
- **Expected outcome:** a small, lawful, buildable `mobile/` package ready for bex features, with reusable theme/providers/controls/D3 charts preserved.
- **Why now:** every later native-client task otherwise pays migration and security cleanup ad hoc; isolating provenance and residue first prevents copied auth/release assumptions from becoming architecture.
- **Render parity:** omitted because this milestone creates no tenant-facing bex behavior or API surface; parity begins when the shell authenticates and consumes bex-api in m2.

## Resolution

Imported a deliberately curated, attributed subset of the source app instead of carrying its product tree wholesale. The resulting Expo 57 package contains the reusable theme/providers, controls, chart primitives, formatting/series/URL utilities, fonts, and navigation shell; it contains no source credentials, release metadata, generated schema, accounting routes, editor/camera permission, or inherited endpoint/project identity. The package is documented, dependency-audited, path-CI-gated, and verified by `yarn test`, Expo Doctor (20/20), and successful iOS/Android exports. Interactive simulator launch remains an environment limitation, not a waived compile/bundle failure.
