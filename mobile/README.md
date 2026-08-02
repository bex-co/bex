# bex mobile

The supervision-first native companion described by
[`docs/ADR048-mobile.md`](../docs/ADR048-mobile.md). This package is an Expo
Router application for iOS and Android. It is intentionally a small shell at
the end of `w11/m1`: Status, Activity, and Sessions are placeholders until the
secure auth/API foundation and feature milestones land.

## What was retained

- Expo Router layout and safe-area patterns.
- Light/dark theme tokens, providers, skeletons, cards, buttons, lists, tabs,
  pickers, progress indicators, and time-range controls.
- Domain-neutral D3/SVG bar, line, and interactive charts.
- JetBrains Mono for logs, identifiers, SHAs, and metrics.
- Formatting, TypeScript, ESLint, and the lightweight unit-test runner.

The provenance and license inventory is in
[`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md). No source authentication,
credential persistence, analytics identity, app-store metadata, or source-domain
feature is retained.

## Commands

```bash
yarn install --frozen-lockfile
yarn start
yarn ios
yarn android
yarn test
yarn bundle:ios
yarn bundle:android
```

`yarn test` runs formatting, TypeScript, ESLint, unit tests, Expo dependency
validation, and public-config validation. Never add credentials to `.env` or
AsyncStorage. Native authentication is owned by `w11/m2` and must follow the
reviewed ADR012/ADR048 contract.
