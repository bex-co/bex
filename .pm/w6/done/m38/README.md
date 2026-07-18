# w6 · m38 — Official CLI `postgres create`: custom database/owner names + fail-closed Datadog flags

**Worker:** worker6 **Goal:** the unmodified official Render CLI can create a bex Postgres instance with explicit `--database-name` and `--database-user` values that become the real, durable CNPG database and owner role, while unsupported Datadog flags fail explicitly instead of returning a false-success no-op. **Status:** done

## Tasks (in order)

| id | title | est | depends_on | status |
| --- | --- | --- | --- | --- |
| t001 | CRD contract: optional immutable physical database + owner-role names with legacy defaults | 1h | — | — **DONE** |
| t002 | Postgres core + REST: validate, persist, read back, and use custom physical names | 1h | t001 | — **DONE** |
| t003 | Operator: project custom names into CNPG bootstrap and preserve them through recovery | 1h | t001 | — **DONE** |
| t004 | GraphQL · MCP · Blueprint · dashboard create parity for the two optional names | 1h | t002, t003 | — **DONE** |
| t005 | Datadog create/update flags: decode and reject explicitly while the external-metrics non-goal remains | 30m | t002 | — **DONE** |
| t006 | Unmodified official CLI + live CNPG proof; update checklist and Postgres ADR | 1h | t004, t005 | — **DONE** |
| t007 | Render parity | 30m | t006 | — **DONE** |
| t008 | Simplify | 30m | t007 | — **DONE** |
| t009 | Test coverage | 1h | t007 | — **DONE** |
| t010 | Closeout | 15m | t008, t009 | — **DONE** |

## Definition of done

Against a real local CNPG installation, an unmodified `render postgres create --confirm --database-name <db> --database-user <role>` call returns 201, stores the requested names as immutable `Database` intent, bootstraps exactly that SQL database and owner role, and returns connection information that successfully authenticates to the requested database. Omitting either field preserves the existing stable-id-derived default independently and keeps legacy CRs byte-compatible. REST, GraphQL, MCP, Blueprint, and the dashboard create flow all reach the same validation/defaulting path. Create/update requests containing `datadogAPIKey` or `datadogSite` return a named unsupported-field 400 rather than 200/201 while the repository's external metrics-integration anti-goal remains in force. Backend/operator tests, generated CRD/deepcopy output, lint, and the official-CLI self-cleaning verifier are green; the CLI checklist and Postgres ADR record the observed contract.

## Source + Goal linkage

- **Source:** user `$pm for w6 to work on postgres create parity of docs/cli-compatibility-checklist.md` (2026-07-18). The checklist's “Real gaps found this pass” records that `CreatePostgresRequest` silently drops `databaseName`/`databaseUser` and server-generates `dpg_<id>`/`dpg_<id>_user`; its `postgres create` matrix marks those two flags and the two Datadog flags open. Rechecked against Render's official CLI reference and Create Postgres API reference on 2026-07-18: both custom-name fields are optional and server-generated when omitted; Datadog monitoring is a credential-bearing agent integration.
- **Goal linkage:** ADR008 pillar 1 (Render-compatible APIs) and the AI-native compatibility thesis: existing Render tooling must be able to express Postgres creation intent without a successful response silently discarding it.
- **Expected outcome:** official CLI users can choose the real initial SQL database and owner role, see those values round-trip everywhere, and connect with the returned URL; unsupported Datadog options fail loudly and safely instead of pretending to work.
- **Why now:** the official-CLI audit isolated this as a reproducible server-side gap after the rest of `postgres create` reached parity. The names cross the API/CRD/operator bootstrap boundary and are effectively immutable after first initialization, so postponing the fix creates more clusters whose physical identity cannot be corrected in place. Datadog implementation is intentionally omitted because `.pm/DO_NOT_DO.md` excludes external log/metric-drain integrations; t005 removes the dangerous silent-success behavior without reopening that product decision. Render parity is included as t007 because this changes tenant-facing REST, GraphQL, MCP, Blueprint, and dashboard behavior.

## Completion evidence

- `scripts/postgres-create-cli-smoke.sh 6` passed on 2026-07-18 with the unmodified Render CLI v2.21.0 against the local CAPD/CNPG cluster: both-custom, database-only, and user-only creates all reached `Ready`, and `current_database()`/`current_user` matched the expected effective identities. Datadog create/update returned the named unsupported error. Every disposable Database was deleted; `dev-6` ended empty.
- The CRD schema was regenerated and installed locally. `make test` passed with a real envtest admission case proving validation plus explicit/defaulted-field immutability. `go test ./...` and `go build ./...` passed for the backend; `go test ./...` passed for types; `make lint` reported zero issues for both Go modules.
- Dashboard `yarn lint`, all 238 test files / 1,501 tests, and `yarn build` passed; generated GraphQL definitions are current. The pinned Markdown Prettier command and `git diff --check` passed.
- No `$simplify` capability was exposed in this session. The required simplification audit was performed manually: backend/operator wrapper implementations were removed, leaving `lego/types/v1alpha1` as the single authoritative validation/default/effective-name contract. No adjacent parity drift remained to file.
