# bex mobile

Expo Router native client for the ADR048 supervision and agent mission-control
experience.

## Boundaries

- Mobile is supervision-first. Do not add service creation, desktop settings,
  blueprint/topology/admin flows, bulk secret management, or Web Shell as a
  primary workflow.
- Environment-variable viewing and editing belong to the dashboard. Mobile
  service details must not render an environment card or request keys or values.
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

`yarn lint` runs ESLint plus framework-aware unused file/dependency analysis.

## Release

The app version lives in `app.json` and `package.json`; `yarn bump
[patch|minor|major|X.Y.Z]` moves both, and `store.config.js` reads the App Store
listing version from `app.json`. Build numbers are **not** in Git: `eas.json`
uses the remote version source, so EAS owns `ios.buildNumber` and
`android.versionCode` and increments them on every production build. The values
left in `app.json` are ignored.

A release is the tag `bex-mobile/vX.Y.Z` on a main commit whose `app.json`
already carries that version ([ADR058](../docs/ADR058-release-engineering.md)
mechanics; mobile versions independently of the platform).
`.github/workflows/mobile-release.yml` then runs `yarn test`, builds both
platforms on EAS with `--auto-submit` (iOS to TestFlight, Android to the Play
`internal` track), waits for every build and submission, and only then
publishes the GitHub release. It needs `EXPO_TOKEN` plus the App Store Connect
API key (`EXPO_ASC_API_KEY_P8`, `EXPO_ASC_KEY_ID`, `EXPO_ASC_ISSUER_ID`) on the
`production-release` environment. Without the ASC key a non-interactive build
cannot regenerate an expired or outgrown provisioning profile.

The same build runs from a logged-in checkout:

```bash
npx eas-cli@24.11.0 build --platform all --profile production --auto-submit --non-interactive
```

EAS archives a fresh clone of the whole repository and honors only committed
ignore rules, so anything large and untracked must be in `.gitignore`, not just
`.git/info/exclude`.

TestFlight and the Play internal track are tester channels. Promoting a build
to App Store review or a public Play track is a separate, deliberate step: it
needs screenshots, the `ASC_*` reviewer contact and demo account from
`.env.template` (`eas metadata:push`), and the physical-device qualification in
`e2e/m2-real-device.md` and
[the push runbook](../docs/runbooks/mobile-push.md#5-release-qualification).
Apple release notes describe only the iPhone and iPad experience; never mention
Android or Google Play in them (App Review 2.3.10).
