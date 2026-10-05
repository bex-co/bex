# w5 · m118 — One source of truth for naming and config-key rules across Go, TypeScript, Kubernetes and CNPG

**Worker:** worker5 **Goal:** Each validation rule has one definition (derived from the platform's own check where one exists), one shared vector file asserted by both Go and TS, and a stable error code. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Give the cron vectors a verdict code asserted by Go and TS | 30m | — |
| t002 | Derive env and secret-file key rules from Kubernetes' own check | 1h | — |
| t003 | One Postgres identifier vector file for types, operator and dashboard | 45m | — |
| t004 | One resource-name rule in types, applied to Blueprint Key Values | 1h | — |
| t005 | Codes for the remaining uncoded validation errors | 1h | t002, t004 |
| t006 | Render parity | 20m | t001, t003, t005 |
| t007 | Simplify | 15m | t006 |
| t008 | Test coverage | 30m | t006, t007 |
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
