# Service Environment page

Live comparison captured on 2026-07-17:

- Render: `https://dashboard.render.com/web/srv-d1iai4be5dus739h1gmg/env`
- bex target: `https://dashboard.bex.co/services/srv-d9bj8s3eg85c7390eb9g/env`
- bex reproducible baseline: the same route component at `http://localhost:<vite>/services/eden-cms-v2/env`, backed by `dashboard/scripts/local-bex.mjs`

The Playwright profile was authenticated to Render. It was not authenticated to the production bex dashboard, so the bex baseline was verified from the route's current source and the local dashboard build rather than by bypassing the login redirect. Screenshots and accessibility snapshots are in the gitignored `.playwright-mcp/` directory.

## Render's interaction model

### Environment Variables

The default state is read-only. Values are masked and can be revealed one at a time. `Export` opens a menu with `Copy env vars` and `Download .env`; either operation necessarily reads all values.

`Edit` changes the entire card into one staged editor:

- Existing keys and values become editable rows, with per-row reveal and delete.
- `Add variable` adds a blank row.
- Its adjacent menu offers `Generated secret`, `Import from .env`, and the newer `Datastore URL` picker.
- `Generated secret` adds a visible, client-generated value under the default key `NEW_SECRET`, so it can be copied before save.
- `Import from .env` opens a modal with a paste textarea, file chooser, `Command+Enter` shortcut, validation, `Add variables`, and `Cancel`.
- One footer commits or discards the whole draft. The split save action offers `Save, rebuild, and deploy`, `Save and deploy`, and `Save only`; `Cancel` discards every staged change.

This is materially different from independent row mutations: users can review a coherent change set, back out safely, and choose its rollout cost.

### Secret Files

The card explains both access paths: the app root and `/etc/secrets/<filename>`. `Add file` enters a staged table with filename, contents, delete, the same three save choices, and Cancel. Its adjacent menu offers multi-file upload. Contents are edited behind a dedicated reveal/editor control rather than occupying the full table row.

### Linked Environment Groups

The service page calls the section `Linked Environment Groups`. With no available group it says so directly and offers `New Environment Group`; a second `Create environment group` action appears beside the page heading. Render's separate creation page asks for a unique group name and initial environment variables and secret files before `Create Environment Group`.

### Responsive behavior

At 390 CSS pixels, Render stacks the card description and actions, keeps action labels readable, contains the key/value table inside the card, and preserves the three-card reading order. Nothing creates page-level horizontal overflow.

## Pre-m44 bex baseline

The bex route already has the core data capabilities: keys/names-only lists, on-demand sensitive reveal, environment-variable replace-all plus single-item writes, secret-file CRUD, server-side generated values, dotenv download, and environment-group link/unlink.

The UX differs in important ways:

- Add/edit/delete mutate immediately and each write rolls pods; there is no card-level edit state, coherent draft, global Cancel, or rollout choice.
- Export downloads only; there is no copy action.
- There is no bulk `.env` paste/file import.
- Generated values are requested server-side and stay unknown until after save.
- Secret files have no upload flow.
- The service-side environment-group card includes workspace-destructive delete and an inline name-only create form rather than routing through the complete create experience.
- At 390 CSS pixels, `CardAction` crushes the title/description into a narrow column. Opening Add variable produces severe horizontal overflow because the key, value, Generate, Save, and Cancel controls remain one unwrapped row.
- The local stub currently omits fields expected by the environment queries, producing Apollo missing-field console errors and weakening browser evidence.

## Implementation contract for w5/m44

The parity milestone keeps sensitive values out of browser persistence and does not reveal them merely to render the view state. A staged batch must update env vars and secret files coherently, project the new Kubernetes Secrets, and cause at most one rollout:

| Choice | Stored/projected | Immediate rollout | Source rebuild |
| --- | --- | --- | --- |
| Save only | yes | no | no |
| Save and deploy | yes | exactly one | no |
| Save, rebuild, and deploy | yes | through the deploy | yes |

Existing public single-item writes remain immediate-roll by default for backward compatibility. The dashboard's new batch contract must preserve unchanged masked values without fetching or resubmitting them.

For a service that has never had a service-local environment Secret, `save_only` records the first projection name in an App annotation instead of changing rollout-bearing `App.spec` references. The operator ignores that metadata-only update under its generation predicate and consumes it on the next deliberate deploy/restart. This closes the subtle first-save case where leaving `spec.restartedAt` alone would otherwise still change the pod template.

## Shipped verification

The production-shaped local route was browser-walked on 2026-07-17 with the real dashboard and `local-bex`, not a component harness:

- Desktop 1440×900 opened read-only with individual values/files masked and no mutation/delete controls. Copy and download export both completed after fail-closed per-value reads.
- One Edit draft staged a previewable 44-character Web Crypto secret, dotenv paste import, and a text-file upload/content edit. Cancel removed the entire draft and the network trace contained zero `PatchServiceEnvironment` calls.
- The rebuild choice produced one successful batch patch followed by an injected deploy-trigger failure. `Retry deploy` sent only a second `TriggerDeploy`; it did not repeat the patch. A separate Save only sent one patch and no deploy.
- At 390×844, view and edit states had `document.documentElement.scrollWidth <= window.innerWidth`; the sticky combined save bar remained inside the card/page width. Dirty navigation opened the accessible discard dialog.
- The page-level Create group action opened the complete dialog with `eden-cms-v2` preselected. A populated group returned already linked, then Unlink and Link both round-tripped through `local-bex`.
- The Environment fixtures emitted no Apollo missing-field problems for env-var/file list, reveal, or batch shapes. The dev build still emits unrelated pre-existing TanStack route-splitting and broad Service/Deploy fixture warnings; t008 explicitly excludes that global cleanup.

Screenshots are `service-environment-desktop.png` and `service-environment-mobile.png` in the gitignored `.playwright-mcp/` evidence directory. The authenticated production bex target still redirects this browser profile to login, so no authentication boundary was bypassed and no claim is made that these uncommitted changes are deployed there.

The final repository gates passed on 2026-07-17:

- `cd lego/backend && go test ./...`
- `cd lego/operator && make test`
- `cd dashboard && yarn typecheck && yarn lint && yarn test` (235 test files, 1,484 tests after rebasing onto current `origin/main`)
- the SNI proxy's asynchronous meter assertion also passed 100 consecutive focused runs after its test-only synchronization fix; production proxy behavior was unchanged

The closing simplification review kept one typed environment draft and one save orchestrator rather than separate env-var/file pipelines, centralized dotenv parsing and diff derivation in pure helpers, and reused the complete environment-group creator. No further consolidation was accepted because collapsing opaque-value, projection, and rollout boundaries would make secret preservation or zero/one-roll semantics less explicit.

## Explicit exclusion

Render now exposes `Datastore URL`, an inline picker that inserts a managed datastore connection string. `.pm/DO_NOT_DO.md` explicitly forbids building that dashboard picker and records the copy/paste or Blueprint alternatives. The live drift is documented here, but w5/m44 excludes it unless the anti-goal is intentionally reopened.
