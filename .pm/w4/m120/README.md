# w4 · m120 — Blueprint manifest env is invisible to the env API and silently shadows dashboard writes

**Worker:** worker4 **Goal:** a `render.yaml` literal `envVars:` entry is visible on the env reads (REST/GraphQL/MCP + Environment tab) flagged as manifest-managed, and a same-key write is refused with a named error instead of succeeding while the process keeps the manifest value. **Status:** todo

## Tasks (in order)

| id   | title                                                                                          | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Reads merge manifest-owned spec.Env, flagged read-only                                         | 50m | —          |
| t002 | Writes to a manifest-owned key fail loudly instead of shadowing                                | 40m | t001       |
| t003 | Dashboard Environment tab renders manifest vars read-only + surfaces the refusal               | 40m | t002       |
| t004 | Render parity + docs                                                                           | 20m | t003       |
| t005 | Simplify                                                                                       | 15m | t004       |
| t006 | Test coverage                                                                                  | 40m | t005       |
| t007 | Live re-probe: manifest env visible, same-key write refused, process value unchanged           | 30m | t006       |
| t008 | Closeout                                                                                       | 10m | t007       |

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
