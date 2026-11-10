# w32 · m297 — Sandbox environment variables, as on Render

**Worker:** worker32 **Goal:** `bex ea sandboxes create --env-var K=V` and the SDK's `create(env=…)` inject variables into the sandbox, write-only, as on Render **Status:** done (verified on production, isolation matrix 42741929387, 2026-11-10)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Design: storage, reserved names and limits, checked against render.com — **DONE** | 30m | — |
| t002 | Accept `env` on REST create, GraphQL and MCP; validate names and limits — **DONE** | 60m | t001 |
| t003 | Pass env to OpenSandbox and keep values off every read, log and event — **DONE** | 40m | t002 |
| t004 | Production check: env probe in the isolation matrix — **DONE** | 40m | t003 |
| t005 | Docs: parity ledger, ea-sandbox and checklists — **DONE** | 20m | t004 |
| t006 | Render parity — **DONE** | 20m | t005 |
| t007 | Simplify — **DONE** | 15m | t006 |
| t008 | Test coverage — **DONE** | 30m | t006 |
| t009 | Closeout — **DONE** | 10m | t007, t008 |

## Definition of done

- On production, `bex ea sandboxes create --env-var K=V` succeeds, and `bex ea sandboxes exec <id> -- printenv K` prints `V`.
- REST/GraphQL/MCP reads never return the value.
- A reserved or invalid name is refused by name (`400 SANDBOX_ENV_INVALID`), naming only the variable.
- The isolation matrix on production is green, including the env probe.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w32`, 2026-11-09, item 1. Render OpenAPI `POST /sandboxes` `env`; bex refuses it as `SANDBOX_ENV_UNSUPPORTED` (`lego/backend/internal/sandbox/service.go:281`, ADR018 sandbox row `env`); w9/067 closed the refusal's wording only.
- **Goal linkage:** Pillar 5 sandboxes and [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first).
- **Expected outcome:** The last everyday create flag the pinned CLI and SDK still refuse works, so Render sandbox scripts that pass env run unchanged.
- **Why now:** The create plumbing already carries env, and m283 established the isolation matrix as a production verification path that doesn't depend on the broken QA login (m284's gate).
- **Render parity:** Included: REST/GraphQL/MCP create surfaces change. Dashboard N/A (bex has no sandbox page).

## Progress — 2026-11-09

- **Shipped:** REST, GraphQL (`env: [EnvVarInput!]`) and MCP (`spawn_sandbox` `env`) accept create-time env. Validation is in `validateSandboxEnv` (`lego/backend/internal/sandbox/service.go`): Render's 100-variable and 256 KB limits, a service's name and value rules (`core.CheckEnvKey`, no NUL, an execve-sized entry), and refused `BEX_`/`EXECD_`/`OPENSANDBOX_` prefixes. The create body is capped at 2 MiB. `env_test.go` covers injection, the values never being echoed, and every refusal.
- **t006 Render parity:** all three surfaces share one validator and one create path, with the same codes and write-only reads. GraphQL adds two refusals of its own (a generated value, a repeated key) because its list shape allows them. Dashboard N/A (no sandbox page; regenerated `schema-types.ts` carries the new argument).
- **t007 Simplify:** applied the reuse of `core.CheckEnvKey`/`ValidEnvValue`, the body cap, a single upper-casing, the MCP limits built from the constants, and the probe-script fixes (`--fail-with-body`, `event: output` parsing, cleanup on a wrongly accepted create, no duplicate reads). Skipped: folding `createWithID`'s parameters into a struct (an existing shape; not worth the churn here).
- **t004 next:** after the deploy, dispatch `isolation-matrix.yml` on production. It now creates sandbox A with `env`, checks `printenv` through bex's exec path, checks that no read returns the value, and checks that a reserved name is refused.

## Closed — 2026-11-10

Production isolation matrix [42741929387](https://github.com/bex-co/bex/actions/runs/42741929387) passed on deploy `2ca11c439`, which carries this milestone; the whole matrix is green.
- `PASS: create-time env is accepted, reserved names are refused, and no read returns a value`
- `PASS: a command run through bex's exec path sees the sandbox's create-time env`
