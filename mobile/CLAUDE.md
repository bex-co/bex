# bex mobile

Expo Router native client for the ADR048 supervision and agent mission-control
experience.

## Boundaries

- Mobile is supervision-first. Do not add service creation, desktop settings,
  blueprint/topology/admin flows, bulk secret management, or Web Shell as a
  primary workflow.
- Never add delete, PITR, failover, workspace deletion, or dangerous permission
  modes. `src/__tests__/mobile-scope-policy.test.ts` enforces the route/action
  vocabulary.
- Authentication must use the reviewed `w11/m2` contract. Never persist OAuth,
  Kratos, or API credentials in AsyncStorage, source code, logs, analytics, or
  crash reports; never use a WebView login.
- Every user-visible string goes through `useTranslations()` and is present in
  both English and Chinese.
- Use theme tokens and `useWindowDimensions`; do not cache a fixed screen width.
- Generated GraphQL belongs to codegen once m2 lands and is never hand-edited.

## Structure

```text
app/                 Expo Router routes
src/common/          providers, hooks, theme, generic utilities and charts
src/components/      reusable controls
src/translations/    en/zh resources
src/__tests__/       cross-cutting policy and utility tests
```

## Required checks

```bash
yarn format:check
yarn typecheck
yarn lint
yarn test:unit
yarn expo:check
yarn bundle:ios
yarn bundle:android
```
