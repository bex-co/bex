# w1 · m175 — Blueprints can copy a sibling service's generated secret (`fromService.envVarKey` → `generateValue`)

**Worker:** worker1 **Goal:** a `render.yaml` in which one service copies another service's `generateValue` variable through `fromService.envVarKey` validates, plans and applies, and both services hold the same generated value. **Status:** done (2026-10-15)

## Tasks (in order)

| id   | title                                                                                        | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Pin down Render's semantics for copying a generated value; record them in ADR049 — **DONE**              | 30m | —          |
| t002 | Validate and plan treat a sibling's `generateValue` as resolved at apply time — **DONE**                 | 1h  | t001       |
| t003 | Apply generates once and seeds the copy with the same value; server-side copy for existing sources — **DONE** | 1h15m | t002 |
| t004 | REST, GraphQL, MCP and the dashboard preview agree; update ADR006 and ADR018 rows — **DONE**             | 40m | t003       |
| t005 | Upstream render-examples fixtures validate and plan unchanged; dev-1 end-to-end apply — **DONE**          | 45m | t004       |
| t006 | Render parity — **DONE**                                                                                | 20m | t005       |
| t007 | Simplify — **DONE**                                                                                     | 15m | t006       |
| t008 | Test coverage — **DONE**                                                                                | 40m | t006       |
| t009 | Closeout — **DONE**                                                                                     | 10m | t008       |

## Definition of done

- The unchanged `render.yaml` files of `render-examples/ghost`, `render-examples/wordpress` and `render-examples/mattermost` (source commits recorded) validate and produce a plan on REST, GraphQL and MCP, with no `envVarKey` refusal. Any other refusal they hit is named and out of scope (for example ADR089 runtime gates).
- A two-service Blueprint where the consumer copies the source's `generateValue` applies through the real env-vars service and against a real OpenBao; both env stores hold the identical value; the value never appears on spec.Env, in a plan or in output. (Amended 2026-10-15: dev-1's base mode has no OpenBao, so dev-1 verified the plan through the real bex-api and the store path ran through `secrets.Service` and a real OpenBao instead; see t005.)
- A second unchanged apply leaves both values unchanged. Render's documented or captured re-sync behavior (t001) is what bex does; if it is unknowable, the Render decision rule in `.pm/w1/README.md` ("keep parity with render") applies and the choice is recorded.
- Copying a value that cannot be known (an unanswered `sync: false` prompt, a reference chain the compiler cannot resolve) still fails before any write with a located error.

## Evidence

- Render's own examples copy a private service's generated password: `render-examples/ghost` (`envVarKey: MYSQL_PASSWORD`, source `MYSQL_PASSWORD: generateValue: true`), `render-examples/wordpress` (`MYSQL_PASSWORD`), `render-examples/mattermost` (`MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`). Verified with `gh api repos/render-examples/<repo>/contents/render.yaml` on 2026-10-15.
- bex refuses it: `docs/ADR006-bex-api.md:248` ("copying a not-yet-minted `generateValue`/reference across services remains explicitly rejected"); `docs/ADR018-render-parity.md:191`. In the compiler a generated value "masks what it overrides" (`lego/backend/internal/apps/deploy.go:2367-2400`, `knownServiceEnv`), so a copy of it has no known value and the Blueprint is refused.
- The `env` → `runtime` alias these manifests use is accepted by the shared compiler without the ADR089 workspace gate (`lego/backend/internal/apps/blueprint_aliases.go`).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w1` 2026-10-15, item 3 (approved by the user the same day); parity-ledger sweep, re-verified against the upstream repositories.
- **Goal linkage:** the Render-alternative core (ADR008; ADR049 `render.yaml` parity): a Render Blueprint deploys on bex unchanged.
- **Expected outcome:** the most common multi-service secret-sharing pattern in Render's own examples imports cleanly instead of refusing the whole file.
- **Why now:** it is a hard refusal on Render's flagship examples, it is independent of ADR089's runtime work, and the compiler code it touches was just stabilized by w8/m64 (code shipped; only its closeout is pending).
- **Render parity included:** this changes Blueprint validate/plan/apply on REST, GraphQL, MCP and the dashboard preview.
- **Related, not taken over:** `w8/blocked/m64` (copy consistency across applies; explicitly excludes this) — rebase on its shipped code, do not reopen it. Running the MySQL/MinIO images live depends on ADR089 and is out of scope.
