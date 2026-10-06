# w5 · m118 — One source of truth for naming and config-key rules across Go, TypeScript, Kubernetes and CNPG

**Worker:** worker5 **Goal:** Each validation rule has one definition (derived from the platform's own check where one exists), one shared vector file asserted by both Go and TS, and a stable error code. **Status:** blocked (t004 step 3 needs the user's decision; t001–t003 and t005–t008 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Give the cron vectors a verdict code asserted by Go and TS — **DONE** | 30m | — |
| t002 | Derive env and secret-file key rules from Kubernetes' own check — **DONE** | 1h | — |
| t003 | One Postgres identifier vector file for types, operator and dashboard — **DONE** | 45m | — |
| t004 | One resource-name rule in types, applied to Blueprint Key Values | 1h | — |
| t005 | Codes for the remaining uncoded validation errors — **DONE** | 1h | t002, t004 |
| t006 | Render parity — **DONE** | 20m | t001, t003, t005 |
| t007 | Simplify — **DONE** | 15m | t006 |
| t008 | Test coverage — **DONE** | 30m | t006, t007 |
| t009 | Closeout | 10m | t008 |

## Definition of done

- `cron-schedule-vectors.json` rows carry a `code`, and both `cron_schedule_vectors_test.go` and `cron.test.ts` assert it.
- `..data` secret-file names are refused at the API with a coded 400 that names the rule. No hard-coded 253 remains (`MaxConfigKeyLength` is gone). Env and secret-file vectors are shared by the core tests and `environment-draft.test.ts`, and `dotenv-import.ts` imports `VALID_ENV_KEY`.
- One Postgres identifier vector file drives the types test, `TestManagedRolesNeverProjectReservedRoles` and the dashboard identifier test, and add-user errors are coded.
- `ValidDatabaseName`, `ValidKeyValueName` and `store.nameRE` are one function with `MaxResourceNameLength`, and a Blueprint Key Value named `Cache_1` gets a validation error, not a 500.
- The remaining uncoded 400s carry codes, the two NAME codes are merged, and the dashboard uses codes instead of English substrings for these rules.

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05, of w4/m168 (`eb55d1e27`), w4/m170 (`faf2ca1f6`), w8/049 (`a4cecaf70`), w4/190 (`6f4ca308a`) and w4/197 (`299767929`). Only cron has a Go↔TS drift guard today, and it checks only valid/invalid. The TS cron copy already crashed the tab (w5/070).
- **Goal linkage:** ADR006/ADR018 consistent API errors; ADR020 identifiers; ADR009/ADR021 datastore naming.
- **Expected outcome:** A rule changed in one place can't drift from its twin, and clients branch on codes, not English.
- **Why now:** Five rule fixes in 24 hours, each with Go/TS copies and magic numbers.
- **Render parity included:** validation errors change on REST/GraphQL/MCP and in the dashboard.
- **Depends on w5/070** (t001) and **w5/071** (t003).

## Evidence — 2026-10-05

**Blocked on one decision (t004 step 3).** Service names still accept resource-ID shapes such as `dpg-` plus 20 characters, while Postgres and Key Value names refuse them (`NAME_RESOURCE_ID_RESERVED`, w8/049) and `AuthorizeApp` resolves an id first. Refusing them for new services is a behavior change the task reserves for the user. Everything else is implemented and shipped.

**Cron (t001).**

- The format refusal is now coded `SCHEDULE_INVALID`, as an empty schedule on create and update is. Never-firing schedules keep `SCHEDULE_NEVER_FIRES`.
- `cron-schedule-vectors.json` gives every row a `code` (null, `SCHEDULE_INVALID` or `SCHEDULE_NEVER_FIRES`) in place of `valid`.
  - `cron_schedule_vectors_test.go` asserts each row's code against `checkCronSchedule`.
  - `cron.test.ts` asserts `cronScheduleProblem`.
  - The `SetCronJob` test asserts the code too.

**Config keys (t002).**

- Env var and secret-file names are checked by Kubernetes' own rules, `validation.IsConfigMapKey` and `content.IsCIdentifier`, through one `configKeyKind` that also carries the reserved names.
- `MaxConfigKeyLength` and the hand-copied charset are gone, and no literal 253 remains for config keys.
- A secret file named `..data` is refused before any store write (`TestDotDotSecretFileIsRefusedBeforeAnyWrite` counts the store's writes). The refusal is coded `SECRET_FILE_INVALID` and names the rule ("must not start with '..'"). Before, the name reached the store, failed the projection, rolled back, and answered an uncoded 400.
- Env keys answer `ENVIRONMENT_VARIABLE_INVALID`, the code ADR013 already documents for the revision-aware update, which now shares the check. Both codes carry params `field` and `maxLength`.
- `config-key-vectors.json` is read by `TestConfigKeyVectors` and `environment-draft.test.ts`.
- On the dashboard side:
  - the secret-file check gained the `..` rule;
  - `dotenv-import.ts` imports `VALID_ENV_KEY`;
  - the env-group copy of the secret-file check delegates to the service one.
- The secrets fake client calls `IsConfigMapKey` instead of hard-coding 253.

**Postgres identifiers (t003).**

- `lego/types/v1alpha1/testdata/postgres-identifiers.json` drives the types test, the operator's `TestManagedRolesNeverProjectReservedRoles` (every well-formed role in the table, declared and tombstoned) and the dashboard identifier test.
- Add-user goes through `ValidateRoleIdentifier`, so an invalid name answers `POSTGRES_IDENTIFIER_INVALID` with `field: name`. Before, it was uncoded.
- The Blueprint parse reuses `ValidateDatabaseIdentifier` and `ValidateRoleIdentifier`, so its refusals are coded too.

**Names (t004, steps 1–2).**

- `appv1alpha1.ValidResourceName` and `MaxResourceNameLength` replace `ValidDatabaseName`, `ValidKeyValueName`, `store.nameRE`/`ValidAppName`, and a fourth copy in `workspaces`. The rule is worded once, as `core.ResourceNameRule`.
- `resourcename.CheckDatastore` is the one Postgres and Key Value name check: create, rename, recovery, and Blueprint parsing.
- A Blueprint Key Value named `Cache_1` now gets a validation error instead of passing validation and failing at apply with a 500 (`TestBlueprintKeyValueNameIsValidated`).
- `resource-names.json` pins `ValidResourceName` and the dashboard's `isValidDnsLabel`, which now replaces three hand-copied regexes (database and Key Value rename rows, workspace forms).

**Codes (t005).**

- `CUSTOM_DOMAIN_INVALID`, `CUSTOM_DOMAIN_RESERVED`, and the 409 `CUSTOM_DOMAIN_IN_USE`; `TestCustomDomainRefusalsAreCodedOnEverySurface` checks REST, GraphQL and MCP.
- `NAME_RESOURCE_ID_RESERVED` now covers service display names too, with `params.field`. `DISPLAY_NAME_RESOURCE_ID_RESERVED` is retired; nothing branched on it.
- GraphQL dropped the code of a coded error a resolver wrapped (graphql-go reads extensions off the returned error only). The formatter now restores it (`TestSanitizeGraphQLErrorsKeepsAWrappedCode`).
- Dashboard:
  - the custom-domain dialog branches on the two codes instead of English substrings;
  - `POSTGRES_IDENTIFIER_RESERVED` gets the localized reserved-name line (en and zh) on create and on add-user;
  - a new `graphQLErrorExtensions` helper also backs `planLimitExtensions`.
- w5/m119 t003 (naming the rejected key in projection refusals) builds on these codes.

**Render parity (t006).**

- bex keeps Render's error envelope. `code` and `params` are its existing superset, and the same refusal answers the same code on REST, GraphQL and MCP.
- Message text changed only where an error became coded: the `bad request: ` and `conflict: ` sentinel prefixes are gone, as on every coded error. The dashboard's `refusalReason` already strips them, and nothing matched them.
- No rule accepts or refuses a different request than before, except two inputs that already failed later:
  - a secret file named `..data` is refused before the write; Kubernetes always refused it at projection, which then rolled the write back;
  - an invalid Blueprint Key Value name is a validation error; it used to pass validation and fail at apply.
- The requests a Render client sends are unaffected: env var names, Postgres identifiers and cron schedules follow the same rules as before.
- The dashboard got localized copy for the reserved-identifier refusal and code-keyed domain refusals.
- No new drift, so no follow-up.

**`/simplify` (t007)**, three reviews. Applied:

- the fourth name-rule copy (`workspaces`) and the remaining hand-worded name messages;
- one `resourcename.CheckDatastore` in place of four copies;
- identifier checks split by kind instead of picking the rule set from a field label, and reused by the Blueprint parse;
- GraphQL's wrapped-code fix;
- the coded empty schedule, and both cron codes as constants;
- the ID-shape message no longer names the wire field;
- `content.IsCIdentifier` in place of a hand-written check, with reserved names folded into the key kind;
- one row type in the operator test, with the doc comment back on its test;
- `gqlSchema` reused, `planLimitExtensions` built on the new helper;
- `isValidDnsLabel` for the dashboard's name checks, a shared `readRepoVectors` test helper;
- stale comments and ADR references (ADR010, ADR017, ADR021).

Skipped:

- the efficiency review's optional fast path for env keys (it would bring back part of the copied rule; 500 keys cost about 143 µs);
- per-surface codes in `cron_schedule_surfaces_test.go` (the vector and `SetCronJob` tests assert the codes).

**Tests (t008).**

- New: the four vector-backed tests, `TestDatastoreNamesPassOneRule` (one table over Postgres and Key Value, create and rename; it replaces the twin per-package tests), `TestCustomDomainRefusalsAreCodedOnEverySurface`, `TestSanitizeGraphQLErrorsKeepsAWrappedCode`, `TestBlueprintKeyValueNameIsValidated`, `TestDotDotSecretFileIsRefusedBeforeAnyWrite`, and the dashboard's `use-access-control.test.ts` and reserved-identifier toast tests.
- Mutation checks, each of which fails its tests:
  - never-firing coded as invalid;
  - a `..` secret file accepted (Go and TS);
  - `none` dropped from the reserved roles (types, operator, TS);
  - the Blueprint Key Value name unchecked;
  - an uncoded reserved host;
  - GraphQL dropping wrapped codes;
  - an uncoded invalid add-user name;
  - the name cap at 31 (Go and TS);
  - the domain dialog without its code branch;
  - the reserved toast dropped;
  - the TS cron problem misclassified.

**Gates.**

- Backend unit suite, operator controller tests and types tests green.
- `make lint` clean on all four modules.
- Dashboard: 4212 tests, typecheck and lint green.
- `/ship` runs the real-dependency backend suite before the push.
