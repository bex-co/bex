# m8 verified invite-link qualification

This gate must pass before claiming that
`https://dashboard.bex.co/invite?invite=<token>` opens the signed bex mobile app.
Simulator routing and valid JSON association files are useful smoke evidence,
but neither proves the OS-to-signing-identity trust chain.

## Signing-identity gate

The repository intentionally contains no guessed signing identity. Obtain both
values from the actual release identities and configure them as GitHub Actions
repository variables:

- `BEX_MOBILE_APPLE_TEAM_ID` — the exact 10-character Team ID that signs
  `co.bex.mobile`.
- `BEX_MOBILE_ANDROID_SHA256_CERT_FINGERPRINTS` — one or more comma-separated
  SHA-256 certificate fingerprints for the keys that actually sign the
  distributed Android app. Include both Play App Signing and direct/EAS release
  certificates when both distributions are supported.

Both variables must be present together. A partial or malformed configuration
fails the dashboard image build. With both absent, the server publishes valid
but deliberately disabled documents (`details: []` and `[]`), so a web fallback
continues to work without falsely claiming a verified native association.

Never derive these values from a debug/simulator build, invent placeholders, or
paste private keys, provisioning profiles, keystores, EAS credentials, invite
tokens, OAuth codes, or session credentials into evidence.

## Hosted-document checks

After deploying the configured dashboard image, verify both URLs without
redirects:

```bash
curl --fail --silent --show-error --dump-header /tmp/aasa.headers \
  --output /tmp/aasa.json \
  https://dashboard.bex.co/.well-known/apple-app-site-association
curl --fail --silent --show-error --dump-header /tmp/assetlinks.headers \
  --output /tmp/assetlinks.json \
  https://dashboard.bex.co/.well-known/assetlinks.json
```

Require HTTP 200, `Content-Type: application/json`, no redirect, the exact
`TEAMID.co.bex.mobile` / `co.bex.mobile` identities, and only the exact
`/invite` path. Validate both bodies with a JSON parser. The public certificate
fingerprints may be compared to the release artifacts, but redact unrelated
headers and do not include an invite token in these requests or evidence.

The pre-implementation production check on 2026-08-02 returned the dashboard
SSR HTML shell with `200 text/html` at both URLs. That is negative baseline
evidence, not a verification pass.

## Physical-device matrix

Use signed release-equivalent builds, one physical supported iOS device and one
physical supported Android device. Expo Go, simulators, and debug certificates
do not satisfy this gate.

| Scenario                                                | Required result                                                                                                                                          |
| ------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| App installed, valid `/invite?invite=<token>`           | The OS opens `co.bex.mobile`; after auth restoration, the app shows the explicit acceptance flow without leaving the bearer token in the visible URL.    |
| App absent                                              | The same HTTPS URL opens the dashboard invitation fallback and supports sign-up/sign-in.                                                                 |
| Wrong path, subpath, HTTP, or custom scheme             | It does not enter the production invite flow.                                                                                                            |
| Missing, malformed, oversized, expired, or reused token | It fails safely according to the invite-flow matrix; no workspace is joined accidentally.                                                                |
| Android verification                                    | `adb shell pm get-app-links co.bex.mobile` reports `dashboard.bex.co` verified for the installed release signature.                                      |
| iOS verification                                        | A fresh install after association deployment opens the app from Mail/Safari without an interstitial; long-press still offers the website fallback.       |
| Signing rotation                                        | Old and new Android release fingerprints overlap in `assetlinks.json` until every supported build has migrated; iOS Team ID remains the release Team ID. |

Record device model, OS version, release build identifier, commit, timestamp,
and pass/fail. Use a disposable workspace and redacted test identity. Association
CDN caches may delay changes, so a stale device result must not be relabeled as
an application pass; capture the hosted document and retry only after the OS
cache refreshes.

## Current release status

Blocked: this checkout contains neither the production Apple Team ID nor an
Android production signing-certificate fingerprint, and no physical-device
matrix has run. Source tests and simulator exports must remain labeled as such.
The local keychain's `PTLM7BZQMM` development Team ID is not sufficient: every
installed provisioning profile belongs to a different bundle ID, so none proves
that team can distribute `co.bex.mobile`.
