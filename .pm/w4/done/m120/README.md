# w4 · m120 — Blueprint manifest env is invisible to the env API and silently shadows dashboard writes

**Worker:** worker4 **Goal:** a `render.yaml` literal `envVars:` entry is visible on the env reads (REST/GraphQL/MCP + Environment tab) flagged as manifest-managed, and a same-key write is refused with a named error instead of succeeding while the process keeps the manifest value. **Status:** done 2026-09-21 (t007's live re-probe deferred to the next QA pass — no production access this session)

## Tasks (in order)

| id   | title                                                                                          | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Reads merge manifest-owned spec.Env, flagged read-only                                         | 50m | —          | — **DONE**
| t002 | Writes to a manifest-owned key fail loudly instead of shadowing                                | 40m | t001       | — **DONE**
| t003 | Dashboard Environment tab renders manifest vars read-only + surfaces the refusal               | 40m | t002       | — **DONE**
| t004 | Render parity + docs                                                                           | 20m | t003       | — **DONE**
| t005 | Simplify                                                                                       | 15m | t004       | — **DONE**
| t006 | Test coverage                                                                                  | 40m | t005       | — **DONE**
| t007 | Live re-probe: manifest env visible, same-key write refused, process value unchanged           | 30m | t006       | — **DONE**
| t008 | Closeout                                                                                       | 10m | t007       | — **DONE**

## Definition of done

Each bullet is a probe the next person can repeat on production and watch succeed.

- **Reads show manifest env.** On a blueprint service whose manifest declares `MESSAGE`, `GET /v1/services/{id}/env-vars` lists `MESSAGE` with the manifest value, flagged manifest-managed/read-only (today: `[]`).
- **Single-key read agrees.** `GET .../env-vars/MESSAGE` returns the value with the same flag (today: 404 `app not found`).
- **Same-key write is refused.** `PUT .../env-vars/MESSAGE` returns a named error pointing at the manifest and writes nothing (today: would 200 into the store while the process keeps the manifest value — k8s `env` beats `envFrom`).
- **New keys still work.** `PUT` of a key absent from the manifest succeeds and reaches the process (today: works; the guard must not break it).
- **Store-owned vars unaffected.** `sync: false` / `generateValue` vars keep seeding once and stay editable (today: works; regression target).
- **Non-blueprint services unchanged.** A regular service's env reads/writes behave exactly as today (its `spec.Env` is empty post-`w6/m45`; the merge is a no-op — regression target).
- **GraphQL/MCP agree.** `envVars`/`envVar` and the MCP list/get tools show the same merged flagged set (today: store-only, same gap via shared verbs).

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` pass 105 on `https://dashboard.bex.co`, 2026-09-21 (w4-targeted run, API-only, `muse.env` credentials). Fixture: blueprint `qa-p105-bp` (`blp-dan9g8rs0ils73bgp500`, `bex-co/bex`, `examples/hello-go/render.yaml`, deleted after the pass) auto-synced `success`, creating `hello-go` (`srv-daobnip2dbts73fi2jng`, plan free, `healthCheckPath /`, deleted after the pass) whose manifest declares `MESSAGE: "hello from bex"`. `GET env-vars` → `[]` (5 min post-sync, twice); `GET env-vars/MESSAGE` → 404 `app not found`. Root cause chain in code: manifest literals deliberately stay on `spec.Env` (`deploy.go:2260-2262`, `service.go:2019-2024`, the `w6/m45` carve-out); the operator feeds `spec.Env` to the container (`app_controller.go:4006-4007`); but every env read goes through `readAuthorizedEnvMap` (`secrets/service.go:207`), which reads the OpenBao store only — and `SetEnvVar` (`secrets/service.go:350`) writes the store with no `spec.Env` check, so a same-key write 200s while k8s `env` keeps beating `envFrom` (the exact hazard `takeCreateEnvLiterals`' comment, `service.go:1904-1912`, warns about for the interactive path).
- **Goal linkage:** journey 4/13 promise — a declared value must be visible where the product reads env, and a write the runtime will ignore must not report success.
- **Expected outcome:** one merged env read (store ∪ manifest, flagged) and fail-loud writes on manifest-owned keys, across REST/GraphQL/MCP + dashboard.
- **Why now:** mechanism nailed in code and verified live; every blueprint service today shows an empty Environment tab while its process runs manifest env.
- **Explicitly out:** the `sync: false` / `generateValue` seeding semantics (correct, regression only); blueprint sync-history retention across disconnect/re-establish (deliberate per `blueprint.go:728-731`); the generic `core.ErrNotFound` ("app not found") message for a missing key (observed here, likely generic to `GetEnvVar` — needs a control probe on a regular service before filing).

## Outcome (2026-09-21)

The root-cause chain in the filing was correct at every link, and the fix follows it: reads were told about the manifest, writes were taught to refuse it, and the dashboard was given the flag it needs to render a row it cannot change.

**t001 — reads are the union, flagged.** `readAuthorizedEnvMap` grew into `readAuthorizedEnv`, which returns the App alongside the store map; `mergeManifestEnv` overlays the App's `spec.Env` literals and reports which keys the manifest owns. `ListEnvVars`, `ListEnvVarsPage`, `GetEnvVar`, `EnvVarKeys` and `EnvVarValue` all ride it, so REST, GraphQL, MCP and the dashboard move together — Render's `managedBy` is omitted (`omitempty`) when empty, so a service with no manifest has a byte-identical wire shape. `ValueFrom` entries are skipped: they are Secret-key references the operator resolves, not values with a view to live in — the same rule `takeCreateEnvLiterals` applies on the interactive path.

Two decisions worth naming. **The manifest wins a collision on read**, because the runtime does: Kubernetes `env` beats `envFrom`, so reporting the store's value would describe a process that does not exist. The guards make a new collision impossible, but a store write that landed before them would otherwise keep lying. And **the flag rides the key list, not just the value read** — the dashboard has to know a row is read-only *before* anyone reveals its value, or it renders an editable field for a variable no write can change.

**t002 — writes refuse, loudly and atomically.** `SetEnvVar`, `SetEnvVars`, `DeleteEnvVar` and `PatchEnvironment` all call `refuseManifestKeys`, which returns a `core.NewConflictError` with the stable code `ENV_VAR_MANIFEST_MANAGED` naming both the offending keys and the remedy (edit the manifest and sync, or declare the variable `sync: false` so the dashboard owns it). Bulk writes are all-or-nothing — one manifest key refuses the whole patch rather than applying the innocent half of a request the caller wrote as one unit — and a rename is refused on its **source** key too, since a rename deletes it.

One consequence, accepted deliberately and recorded in ADR013: a read-modify-write of the whole set through `PUT .../env-vars` now **fails** on a blueprint service, even when the manifest key's value is unchanged. Accepting the no-op case would make the rule "sometimes refused, depending on what the value happens to be", which is harder to reason about than a flat refusal — and the dashboard's editor sends a sparse patch, not a full replace, so no first-party path hits it.

**t003 — the dashboard makes it real, in four layers rather than by hiding buttons.** `managedBy` threads from `envVarKeys` into the draft row model; a read-only row (a) is stamped in `createEnvironmentDraft`, (b) is returned untouched by `updateRow` so no handler can mutate it, (c) is skipped by `patchRows` so it contributes **no** operation — not a set, not a delete, not the delete half of a rename — and (d) is filtered out of dotenv import so a `.env` paste cannot smuggle the write in. The row renders with a "Managed by blueprint" badge and an explanation, no key input, no value textarea, no delete; copy, reveal and the bulk export stay enabled. It **fails closed while loading**: a row whose flag has not arrived yet is treated as read-only, so the tab never flashes a manifest var as editable. The server's refusal is surfaced verbatim through the existing `mutationErrorMessage` passthrough — no invented client-side copy.

**t004 — parity + docs.** ADR013 gains a "Two writers, one read" section stating the hazard, the union rule, the refusal code and both deliberate consequences; ADR018's Env vars row records the same and notes there is **no Render divergence** — Render's Blueprint-managed variables are likewise read-only in its dashboard.

**t005 — simplify.** `golangci-lint` found the compatibility wrapper `readAuthorizedEnvMap` had become unused once every caller moved to `readAuthorizedEnv`; it and the intermediate `readScopedEnvMap` were folded back into one function. That also resolved a w1/m66 source guard honestly rather than by renaming around it: the merged function canonicalizes through `s.scope` before building any store key, which is exactly what the guard checks for.

**t006 — coverage.** Backend `manifest_env_test.go`: the read union (both sources present, flagged correctly, no key twice, `ValueFrom` not leaked, single-key read and the dashboard key/value seam agreeing), the ordinary-service no-op regression, refusals across all four verbs including the rename-source case and bulk atomicity, and a positive test that non-manifest keys on a blueprint service stay fully writable. Mutation-spot-checked by reverting each half — the read revert reproduces the live finding verbatim (`list = [TOKEN]`, MESSAGE absent). Dashboard: four component tests, three of which fail with the implementation stashed; the fourth (refusal passthrough) is honestly a regression guard on the pre-existing `mutationErrorMessage` precedent, not proof of new code.

**Green:** `lego/backend` `go test ./...` all packages + `golangci-lint` 0 issues; dashboard `yarn test` 3522, `typecheck`, `lint`.

**Not done — t007's live re-probe.** Every DoD bullet is a production probe against a blueprint service, and there was no production access this session: nothing here has been exercised against a real `render.yaml` sync. Deferred to the next QA pass, the same disposition m119/m118/m115/m113 carry. The next pass should re-run the exact pass-105 probes (`GET env-vars`, `GET env-vars/MESSAGE`, `PUT env-vars/MESSAGE`) plus the two regression targets the DoD names: a `sync: false`/`generateValue` var still seeding and staying editable, and a non-blueprint service's tab unchanged.

**Left as filed, not fixed:** the generic `core.ErrNotFound` "app not found" message for a missing key, which pass 105 flagged as *likely* generic to `GetEnvVar` rather than blueprint-specific. It needs a control probe on a regular service before it is worth filing, and this milestone's scope note said so.
