# w5 · m37 — Registry-credential attach: complete the private-image deploy flow

**Worker:** worker5 **Goal:** the dashboard's registry-credential settings panel stops being a dead end — a user can select a stored credential when deploying a private image, and change/clear it in service settings, matching Render's flow. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Plumb `registryCredentialId` through the dashboard's create/patch GraphQL ops — **DONE** | 30m | — |
| t002 | Credential selector on the image source tab of the create flow — **DONE** | 45m | t001 |
| t003 | Attach/detach a credential in service settings' image section — **DONE** | 45m | t001 |
| t008 | Show the bound registry credential on the service overview/detail page — **DONE** | 30m | t001 |
| t004 | Render parity — compare against Render's private-image create + settings flows — **DONE** | 30m | t002, t003, t008 |
| t005 | Simplify — `/simplify` over the milestone's diff — **DONE** | 20m | t004 |
| t006 | Test coverage — meaningful tests for the shipped behavior — **DONE** | 30m | t004 |
| t007 | Closeout — verify DoD, sync status, move to `done/` — **DONE** | 15m | t006 |

## Definition of done

From the dashboard, a user creates a service from a private image while selecting a stored registry credential, and later changes or clears that credential in service settings; the value round-trips through GraphQL (visible on re-query). Verified against a dev-N stack once the backend binding exists.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 16, 2026-07-15 — dashboard-gap mine over freshly shipped backend surfaces: `dashboard/src/features/registry-credentials/` ships full credential CRUD in account settings, but `grep registryCredential` across `features/services/` and `routes/services.new.tsx` returns nothing — no way to _use_ a credential at either of Render's two attach points (create-from-image, service settings). `w6/m31` is scoped to REST/GraphQL/MCP/CLI parity only; its DoD names no dashboard work.
- **Goal linkage:** Render dashboard parity for private-registry deploys — the last unowned piece of the `w2/m14` registry-credentials feature (`docs/ADR018-render-parity.md` registry-credentials row).
- **Expected outcome:** private-image deploys are completable end-to-end from the UI; the settings credential panel stops being a dead end (the same dead-end class `w5/m35` targets).
- **Why now:** `w6/m31` is in flight — sequencing this now lands the UI right behind the API binding instead of becoming a future round's dead-end find. **Gate:** t001 requires `w6/m31`'s GraphQL create/patch binding (`registryCredentialId` on the service mutations) to have landed; if picked up first, start with t002/t003's UI scaffolding against the generated schema only after that binding exists — coordinate, don't stub.
- **Render parity:** included (UI surface; t004 compares the real Render flows).

## Resolution

Completed 2026-07-15. The backend binding and initial create pickers had landed upstream through `w6/m31` and `w6/m34`; this milestone reconciled that work instead of duplicating it, extracted one shared selector, and completed the missing existing-service and read-only paths.

- Existing Image and repository-Docker create flows use the shared workspace credential picker. “No credential — public image” deliberately sends explicit `""`, matching the current backend's no-auth semantics; omission remains the legacy preserve/auto-match behavior.
- Eligible existing-image and repository-Docker services expose the same picker in Settings with change and clear success/error states. Native/buildpack/static repository sources do not show an inapplicable control.
- The service query projects `registryCredentialId`, and the detail header resolves the stored credential's human name with a link to the account credential panel. Clearing the binding removes the fact.
- The dev-5 browser proof created an image service with credential A, confirmed A through a fresh GraphQL query, changed to credential B in Settings, confirmed B in both a fresh query and the header, then cleared to explicit empty and confirmed the header disappeared. The temporary service, both credentials, their secrets, and the identity were removed.
- The simplify pass retained the shared selector/hook split and added a service-id key so draft state cannot leak across route-param changes. After reconciling the latest `origin/main`, validation passed with `yarn lint`, `yarn typecheck`, 211 Vitest files / 1,226 tests, `yarn build`, `git diff --check`, and the live dev-5 walkthrough.

`docs/ADR018-render-parity.md` and `docs/render-artifacts/registry-credential-service-binding.md` now record the completed dashboard coverage and distinguish this binding proof from the existing authenticated private-pull and private-Docker-base live proofs.
