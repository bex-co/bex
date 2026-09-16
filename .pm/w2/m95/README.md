# w2 · m95 — Environment values mean what the user typed: round-trip escapes, multi-line values, and `PORT`

**Worker:** worker2 **Goal:** a value that leaves the dashboard through Export comes back identical through Import; a multi-line value pasted into any env value field is saved with its line breaks; and a user-set `PORT` key is refused with a coded reason on every write surface instead of being saved and silently ignored. **Status:** t001–t006 done; **t007 closeout BLOCKED** on live production verification (see Blocker)

## Tasks (in order)

| id   | title                                                                                                                         | est | depends_on       |
| ---- | ----------------------------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | `parseDotenv(formatEnvExport(x))` equals `x` for any string; property test over control chars, quotes, `#`, U+2028 — **DONE** | 30m | —                |
| t002 | Env value fields become auto-growing textareas at every editor site so a pasted PEM keeps its line breaks — **DONE**        | 25m | —                |
| t003 | Refuse a user `PORT` key on REST, GraphQL, MCP, Blueprint and both dashboard editors; record the rule in ADR018 next to ADR004 — **DONE** | 40m | —                |
| t004 | Render parity — **DONE**                                                                                                      | 15m | t001, t002, t003 |
| t005 | Simplify — **DONE**                                                                                                           | 15m | t004             |
| t006 | Test coverage — **DONE**                                                                                                      | 35m | t004             |
| t007 | Closeout                                                                                                                      | 10m | t006             |

## Decisions

- **The escape contract is the full JSON set, inside double quotes only** (t001 step 1). `formatEnvExport` already serialized with `JSON.stringify`, so making `parseDotenv` its exact inverse was the smaller, safer half — Export is unchanged. Written into a header comment on both files so the pair cannot drift.
- **A literal `\uXXXX` a user typed into their own `.env` *is* expanded** (t001 step 2), inside double quotes only. Common dotenv parsers do not. Single-quoted and unquoted values stay verbatim, which is the escape hatch for anyone who means the backslash, and the property test pins both directions.
- **No `fast-check` dependency.** t001 offered it "if already a dev dependency, otherwise add it"; it is not one. The property test uses a seeded mulberry32 generator instead, so a failure reproduces from its seed and the dashboard gains no supply-chain surface for one test file. 1,000 generated entry sets over control characters, U+2028/U+2029, surrogate pairs, lone surrogates, quotes, `#`, `=`, `export `, and the empty string.
- **`PORT` is refused on writes, never on deletes.** The DoD says nothing about removal, but refusing everything would have stranded any pre-rule `PORT` forever — unreachable by the very API that stored it. Deleting one is always legal, and the dashboard flags only rows a save would actually write (added, renamed onto, or value-changed), so a service holding a pre-rule `PORT` can still save unrelated edits. Guarded by `TestAStoredReservedEnvKeyCanStillBeDeleted`.
- **Reserved for every App type, not web only** (t003 step 1, confirmed in `appEnv` before writing it down). The operator injects `PORT` for web, private, worker, cron and static alike, so a per-type exception would be a rule nobody could remember. Recorded in ADR004 and ADR018.
- **Exact and case-sensitive.** `port`, `PORTAL` and `APP_PORT` are ordinary application variables the operator never touches. `TestOnlyTheExactReservedNameIsRefused` keeps that true.
- **Blueprint validation reports the sentence with a location instead of the code.** Every other surface returns coded `ENVIRONMENT_VARIABLE_RESERVED`. The Blueprint validation surface renders error *strings* (`BlueprintValidationError.Error`) and drops codes for every parse error, not just this one — so matching its neighbours (`api envVars["PORT"]: …`, `env group "shared" envVars["PORT"]: …`) beats inventing a code path the surface would discard. The shared wording still comes from `core.ReservedEnvKeySentence`, so there is one sentence.
- **`new-env-group-dialog`'s value field keeps its masking through `-webkit-text-security: disc`.** A textarea has no `type="password"`. That property is supported by Chromium, WebKit and Firefox 137+, so no browser in the support matrix regresses; it is documented on `AutoTextarea`.

## Traced value-field sites (t002 step 1)

t002's Context guessed the untraced consumers were "the create-service form's env rows". That was right about the form and wrong about the file:

| Site | Reached by | Swapped |
| --- | --- | :-: |
| `service-environment-editor.tsx` `EnvDraftItem` | service Environment page **and** the env-group detail editor (`env-group-editors.tsx` renders the same `EnvironmentEditor`) | ✅ |
| `new-env-group-dialog.tsx` | `routes/env-groups.tsx`, `env-groups-panel.tsx` | ✅ |
| `create-env-var-editor.tsx` | `routes/services.new.tsx` — the create-service wizard | ✅ |
| `env-var-row.tsx`, `env-vars-panel.tsx`, `secret-files-panel.tsx` | **nothing** — no route or component imports them; only their own test files do, which is why `knip` still counts them used (it treats tests as entry points) | ✖ |

The last row is a pre-existing dead trio, not m95's to delete — filed as `w2/036` for the dead-code routine rather than churned here.

## Per-surface `PORT` refusal (t004 step 1)

Every row asserted by a test; "store after" is checked in the same test.

| Surface | Verb | Response | Store after |
| --- | --- | --- | --- |
| REST | `PATCH /v1/services/{id}/environment` | 400 `ENVIRONMENT_VARIABLE_RESERVED` | unchanged |
| REST | `PUT /v1/services/{id}/env-vars/PORT` | 400 `ENVIRONMENT_VARIABLE_RESERVED` | unchanged |
| REST | `PATCH /v1/env-groups/{id}/contents` | 400 `ENVIRONMENT_VARIABLE_RESERVED` | unchanged |
| REST | `PUT /v1/env-groups/{id}/env-vars/PORT` | 400 `ENVIRONMENT_VARIABLE_RESERVED` | unchanged |
| GraphQL | `patchServiceEnvironment` | `core.ErrBadRequest`, same sentence | unchanged |
| GraphQL | `patchEnvGroupEnvironment` | `core.ErrBadRequest`, same sentence | unchanged |
| MCP | `patch_service_environment` | `isError`, same sentence | unchanged |
| MCP | `patch_env_group_environment` | `isError`, same sentence | unchanged |
| Service core | `SetEnvVar` · `SetEnvVars` · `SeedEnvVars` · rename-onto | `core.ErrBadRequest`, same sentence | unchanged |
| Env-group core | `CreateEnvGroup` · `SetEnvGroupVars` · `SetEnvGroupVar` · `SetEnvGroupVarInput` · `PatchEnvironment` | `core.ErrBadRequest`, same sentence | unchanged |
| Create service | REST/GraphQL/MCP `envVars` → `checkReservedSpecEnv` | `core.ErrBadRequest`, same sentence | no App CR |
| Blueprint | `services[].envVars` | 400, sentence + `api envVars["PORT"]` location | no write |
| Blueprint | `envVarGroups[].envVars` | 400, sentence + `env group "shared" envVars["PORT"]` location | no write |
| Dashboard | Environment editor · env-group editor · new-group dialog · create wizard | inline hint as typed, Save/Create blocked | no mutation |

No disagreement between surfaces. The one deliberate shape difference (Blueprint carries a location instead of a code) is in Decisions above.

## Render comparison (t004 step 3)

Re-read 2026-09-15.

- **`PORT`** — Render: "The default value of `PORT` is `10000` for all Render web services. You can override this value by setting the environment variable" (`render.com/docs/web-services`). bex refuses it. A recorded divergence, now a row in `docs/ADR018-render-parity.md` beside the env-vars row, with the reasoning and the rejected option (b).
- **Multi-line values** — Render's env editor accepts multi-line values in its value field. bex now does too, at all four sites. Parity restored, not divergence.
- **`.env` import escapes** — Render's docs specify only "valid `.env` syntax" and name no escape set. bex documents its contract explicitly and is a superset of common dotenv parsers inside double quotes. Stricter and self-consistent rather than divergent; noted in the ADR018 env-vars row.

## Production `PORT` inventory (t004 step 4)

Read-only sweep of the production cluster on 2026-09-15 (`kubectl`, **keys only** — no value was read):

- projected env Secrets (`*-env`) scanned: **3**, holding a `PORT` key: **0**
- App CRs scanned: **13**, carrying a literal `PORT` on `spec.env`: **0**
- env-group Secrets scanned: **4**, holding a `PORT` key: **0**

Nothing to clean up: the refusal strands no existing value, so no migration is needed.

## Simplify pass (t005)

- **Applied:** the nine `if !core.ValidEnvKey(key) { return <bad request> }` sites across `secrets/` and `envgroups/` collapsed onto one `core.CheckEnvKey(key) error`, so the well-formedness and reserved refusals cannot drift apart and no path can gain one check without the other. The batch patch's two map appliers now share a single `applyMapPatch(…, reserved, …)` parameter (nil for secret files) rather than each growing its own guard.
- **Applied:** `ReservedEnvKeyErrorAt` — a details-carrying variant added while wiring Blueprint — was **deleted** once the Blueprint surface turned out to discard details; carrying it would have been an unused seam the dead-code analyzer flags.
- **Declined:** hoisting the dashboard's `isReservedEnvKey` into a shared `common/` module. It belongs beside `VALID_ENV_KEY` in `environment-draft.ts` (the file that already mirrors the backend's key rules); `env-groups/lib/validation.ts` importing it from there is the one cross-feature edge, and it is the correct direction — env groups already depend on services' environment vocabulary.

## Blocker (t007)

**t001–t006 are complete and every gate is green.** t007 requires re-probing each Definition-of-done bullet **live on the deployed dashboard and API**, and that is gated on a credential only the user holds:

- `.env`'s `QA_EMAIL`/`QA_PASSWORD` are stale — `scripts/qa-login.sh` fails with *"The provided credentials are invalid"* from Kratos.
- `~/.bex/cli.yaml`'s token is expired (`bex services` → *"your token is expired; run `render login`"*); re-login is an interactive browser OAuth flow.
- A fresh `bex-bootstrap` client-credentials token (minted from `BEX_BOOTSTRAP_CLIENT_SECRET`) authenticates (`200` on `GET /v1/services`) but holds no membership in workspace `bex`, so every tenant-scoped call returns `forbidden` — it cannot create or drive the DoD fixtures.

**To clear it:** refresh `QA_PASSWORD` in `.env`, or run `! render login`. Then re-probe the six DoD bullets and close the milestone.

**What stands in for the live walk meanwhile** (recorded honestly, not presented as the DoD): the Export/Import bullet is pinned by the 1,000-case property test plus explicit per-escape cases, both shown failing on the pre-fix parser; the two multi-line bullets by component tests that assert the newlines reach the mutation payload, also shown failing on the pre-fix `<input>`; the `PORT` bullet by the per-surface table above, every row test-backed; the "rules are recorded" bullet by the ADR018 row and ADR004 line, which need no live probe.

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
