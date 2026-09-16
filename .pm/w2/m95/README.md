# w2 · m95 — Environment values mean what the user typed: round-trip escapes, multi-line values, and `PORT`

**Worker:** worker2 **Goal:** a value that leaves the dashboard through Export comes back identical through Import; a multi-line value pasted into any env value field is saved with its line breaks; and a user-set `PORT` key is refused with a coded reason on every write surface instead of being saved and silently ignored. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                         | est | depends_on       |
| ---- | ----------------------------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | `parseDotenv(formatEnvExport(x))` equals `x` for any string; property test over control chars, quotes, `#`, U+2028           | 30m | —                |
| t002 | Env value fields become auto-growing textareas at every editor site so a pasted PEM keeps its line breaks                    | 25m | —                |
| t003 | Refuse a user `PORT` key on REST, GraphQL, MCP, Blueprint and both dashboard editors; record the rule in ADR018 next to ADR004 | 40m | —                |
| t004 | Render parity                                                                                                                 | 15m | t001, t002, t003 |
| t005 | Simplify                                                                                                                      | 15m | t004             |
| t006 | Test coverage                                                                                                                 | 35m | t004             |
| t007 | Closeout                                                                                                                      | 10m | t006             |

## Definition of done

Run each bullet on the production dashboard against a throwaway free web service (`examples/hello-go`), created and deleted inside the run, with the API responses captured:

- **Export/Import round trip is byte-exact.** `PUT /v1/services/<srv>/env-vars/CTRL` with a value holding a real ESC byte (U+001B) between `esc` and `end`. Environment → Export → "Copy env vars", then Edit → Add variable → "Import from .env" → paste the copied text → "Add variables" → Save and deploy. `GET /v1/services/<srv>/env-vars/CTRL` returns the value with the ESC byte intact. At filing time it returned `escu001bend` (the JSON escape read as letters).
- **A pasted multi-line value keeps its line breaks.** Edit → Add variable → key `MULTI`, paste three lines (`first`, `second`, `third`) into the value field → Save and deploy. `GET …/env-vars/MULTI` returns the three lines joined by newlines. At filing time the field flattened it to `first second`. The field itself shows three lines before save.
- **An imported multi-line draft displays truthfully.** Import a `.env` line `PEM="-----BEGIN KEY-----\nline-one\nline-two\n-----END KEY-----"` (with `\n` escapes); the draft row shows the value on four lines, not run together, and saves unchanged (control: this value already saved correctly at filing time).
- **`PORT` is refused with a code on every surface.** Each of the following returns a 400-class refusal whose body names the service port setting, and no `PORT` key is stored afterwards:
  - REST `PUT /v1/services/<srv>/env-vars/PORT {"value":"8080"}`;
  - REST `PATCH /v1/services/<srv>/environment` with `{"key":"PORT","value":"8080"}` in the batch;
  - REST `PUT /v1/env-groups/<evg>/env-vars/PORT` and the group batch `PATCH …/contents`;
  - GraphQL `patchServiceEnvironment` and MCP `patch_service_environment` with the same key;
  - `POST /v1/blueprints/validate` on a `render.yaml` whose service `envVars` lists `PORT`.
  - The dashboard create form and the Environment editor show the inline hint the moment `PORT` is typed as a key, and Save is refused with the server's reason if it is bypassed.
- **The rule is recorded.** `docs/ADR018-render-parity.md` carries a `PORT` divergence row beside the env-vars row citing ADR004:152, and states whether background workers and cron jobs share the rule.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w1` 2026-09-15, proposal 2, absorbing [w1/done/099](../../w1/done/099.md) (export escapes come back as letters; a pasted multi-line value is flattened; major) and [w1/done/096](../../w1/done/096.md) (`PORT` saved but silently ignored). Both were found live in the 2026-09-14 `/qa-find-bugs` hunt.
- **Goal linkage:** pillar 2 (agent-readable, deterministic state): a value must survive the dashboard untouched, and a saved key must mean what it says. ADR018 rows 112 and 113 claim full env-var/secret-file parity while `PORT` is an unrecorded divergence from Render's "set `PORT` to pick the port".
- **Expected outcome:** no secret is silently altered on the way in (JSON escapes and newlines survive export → import and paste → save), and a migrating Render user learns immediately, with a code, that bex owns `PORT` and where to change the service port instead.
- **Why now:** 099 corrupts secrets on save with no warning, the worst class of dashboard defect, and its fix is contained to two dashboard libraries plus the value fields. `PORT` is decided as **option (a)**: keep the operator-owned invariant (a user `PORT` in an env group would otherwise silently change routing and health checks for every linked service) and refuse rather than honor. Render parity is **included**: t003 touches REST, GraphQL, MCP, Blueprint validation and both dashboard editors, and t001/t002 change what the dashboard stores.
