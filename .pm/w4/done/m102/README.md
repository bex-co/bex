# w4 · m102 — Blueprint validation names the field that is actually wrong, and the shipped static-site example validates

**Worker:** worker4 **Goal:** a blueprint author who makes one mistake is told which key caused it, instead of being sent around a three-message loop that never names the offender — and the repository's own static-site example passes the validator it is meant to demonstrate. **Status:** done

## Tasks (in order)

| id   | title                                                                            | est | depends_on                 |
| ---- | -------------------------------------------------------------------------------- | --- | -------------------------- |
| t001 | Fix `examples/static-site/render.yaml` and guard every shipped manifest in CI — **DONE** | 40m | — |
| t002 | Pick the `anyOf` branch by its discriminator, not by leaf count alone — **DONE** | 60m | — |
| t003 | Speak manifest vocabulary in blueprint-surfaced create-validator errors — **DONE** | 45m | — |
| t004 | Blast-radius + control-case regression tests for branch selection — **DONE** | 45m | w4/m102/t002, w4/m102/t003 |
| t005 | Render parity sweep over the changed surfaces — **DONE** | 30m | w4/m102/t001, w4/m102/t004 |
| t006 | Simplify pass over this milestone's changes — **DONE** | 30m | w4/m102/t005 |
| t007 | Test coverage for the shipped behavior — **DONE** | 40m | w4/m102/t005 |
| t008 | Closeout — **DONE** | 15m | w4/m102/t007 |

## Definition of done

Each bullet is a request the next person can re-run against production and watch succeed.

- **The repository's static-site example validates.** `blueprintPreview(repo:"https://github.com/bex-co/bex", branch:"main", path:"examples/static-site/render.yaml")` returns `validation.valid: true` with a non-null `plan`. Today it returns `valid: false`, `plan: null`, and one error.
- **All seven shipped manifests validate, and CI keeps them that way.** A test compiles every `render.yaml`/`bex.yml` tracked in the repo through the same validator the API uses and fails on any that does not validate. Today: 7 manifests, 6 valid, 1 invalid (`examples/static-site/render.yaml`).
- **One wrong key is named as that key.** For the manifest below — the example's exact pre-fix shape — the reported error names **`plan`**, the property that is actually disallowed on a static service, not `staticPublishPath`:

  ```yaml
  services:
    - name: qa-static-bp
      type: web
      runtime: static
      repo: https://github.com/bex-co/bex
      rootDir: examples/static-site
      branch: main
      staticPublishPath: .
      plan: free
  ```

  Today: `at '/services/0': additional properties 'staticPublishPath' not allowed`.

- **Obeying the error converges instead of looping.** Removing the field the error names must not produce a new error naming a third key. Today the loop is closed and verified:
  1. the manifest above → `additional properties 'staticPublishPath' not allowed`
  2. remove `staticPublishPath` → `service "qa-static-bp": bad request: publishPath is required for a static_site`
  3. add `publishPath: .` → `additional properties 'publishPath' not allowed`

  None of the three names `plan`, and removing `plan` alone makes the original manifest valid (verified).

- **Blueprint errors use manifest keys.** A `static_site` blueprint missing its publish directory is told to set **`staticPublishPath`** — the key the schema accepts — not `publishPath`, which `docs/ADR006-bex-api.md:167` records as rejected input and which the validator does reject (verified, step 3 above). The REST/MCP create paths keep their own `publishPath` wording, because that is their field name.
- **The enum error lists the real set.** `type: nonsense` currently answers `value must be one of 'web', 'worker', 'pserv'`, yet `type: cron` validates (verified). The message must enumerate every accepted value, or a cron-job author concludes blueprints cannot express cron jobs.
- **Discriminated branches win.** A service declaring `type: web` + `runtime: static` reports only `staticService`'s complaints; `type: cron` reports only `cronService`'s; and a service matching no discriminator still reports something actionable rather than an arbitrary branch's. Asserted per branch.
- **An env-var mistake is reported as an env-var mistake.** A valid `serverService` whose only fault is inside one `envVars` entry reports that entry — not a `redisServer` complaint. Five such manifests (bogus field inside `fromDatabase`; invalid `fromDatabase.property`; bogus field beside a literal `value`; `fromService` missing `type`; both `value` and `fromDatabase` on one entry) must each name their own fault, and must not all return the same error text. Today **all five** return the identical three `redisServer` errors — `missing property 'ipAllowList'`, `additional properties 'runtime', 'repo', 'branch', 'envVars' not allowed`, `type must be one of 'keyvalue', 'redis'` — while the same manifests with the fault removed are `valid: true` (measured pass 9; see t002).

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 2026-09-13 pass 5, journey 13 (Blueprints), against production `api.bex.co` as workspace `bex` / `tea-d98210cbbpdc73dcrkvg`. Every probe is a `validateBlueprint` / `blueprintPreview` GraphQL call with its full response pasted into t001–t003 — **no resource was created, synced or applied for this finding**, so the evidence is re-runnable by anyone with a session and needs no fixture.
- **Goal linkage:** `docs/ADR049-render-yaml-parity.md` and `docs/ADR018-render-parity.md` — the blueprint schema is a SHA-pinned snapshot of Render's own `render.yaml` schema (`RenderBlueprintSchemaSHA256`, fetched 2026-08-02), so rejecting `plan` on a static service is **faithful to Render** and the schema is not the thing to change. `docs/ADR029-static-sites.md` owns the static-site contract; `docs/ADR006-bex-api.md:167` fixes the `staticPublishPath` vs `publishPath` vocabulary.
- **Expected outcome:** the first blueprint a user writes from this repository's own example works, and when they do make a mistake the error points at it. Blueprints are the IaC entry point for pillar 4 (deploy-from-chat) — an agent handed a `render.yaml` cannot iterate against messages that name the wrong key.
- **Why now:** the failing manifest is the one a user copies to learn the static-site shape, and the misattribution is not cosmetic — following the message makes the manifest *worse*, and the loop has no exit without reading the JSON schema. Both are cheap to fix and the mechanism is already pinned to a line.
- **Render parity task included:** yes — blueprint validation is a user-facing contract exposed on GraphQL (`validateBlueprint`, `blueprintPreview`), MCP, and the dashboard's Blueprints surface, and the vocabulary question is precisely a parity question.

## Dedupe

- `w8/m19/t003` (done) made static **`buildCommand`** accepted and driving the build. Different property, different failure; it did not touch `plan`, branch selection, or the example manifest.
- `w7/m30` (done) built a small **OpenAPI** subset validator for a different surface; it is not this JSON-Schema path.
- `w6/064` (done) was about a blueprint **apply result** omitting created env groups — a response-shape bug on the apply side, not validation.
- Searched `.pm` open and `done/` for `staticPublishPath`, `anyOf`, and "closest alternative". `.pm/DO_NOT_DO.md` has no matching anti-goal.
- **Not already fixed on `main`:** `blueprint_compiler.go:1089-1097` still selects by leaf count with no discriminator tiebreak; `service.go:2456` still emits `publishPath is required for a static_site`; `examples/static-site/render.yaml:15-16` still carries `staticPublishPath: .` followed by `plan: free`.

## Verified this run, and not claimed

- **Verified live:** all four error strings in the DoD; that removing `plan` alone makes the manifest valid; that `type: cron` validates while the enum message omits it; that 6 of the repo's 7 manifests validate (`find . -name "render.yaml" -o -name "bex.yml"` → 7, all previewed individually); that `staticService` in the pinned schema allows `staticPublishPath` and has no `plan` property, and that the capability registry has 23 `staticService` entries and none for `plan`.
- **Traced in code, not observed:** that the tie between the `serverService` and `staticService` branches is what selects the wrong one. `blueprintSchemaLeafCount` returns 1 for each single-additional-property failure and `closest` starts at `Causes[0]`, with `#/definitions/resources.services.items.anyOf` ordered `[redisServer, cronService, serverService, staticService]` — so a tie keeps the earlier branch. The leaf counts were not instrumented at runtime; t002 must confirm them before relying on this explanation.
- **Not exercised:** a blueprint **apply** (`syncBlueprint`) of any of these manifests. The manifests in `examples/` use unprefixed names (`web`, `worker`, `db`, `static-site`) that cannot be namespaced to a `qa-` fixture, so applying one on production was out of bounds for this run. Journey 13's "plan diff matches what apply does" half therefore remains **unverified** — the plan side is verified, the apply side is not. A dev-stack run (`scripts/dev-env.sh 4 up`) is the right place for it.
- ~~The env-var `anyOf` forms … no env-var misattribution was reproduced this run.~~ **Probed 2026-09-13 pass 9 — reproduced, and worse than the static-site case that motivated this milestone.** Five different env-var faults in an otherwise-valid `serverService` all return the same three `redisServer` errors and none names the real mistake; the clean controls validate. It also corrected the mechanism: leaf-count selection systematically prefers the branch that failed *shallowest*, because the correct branch accumulates extra leaves by reaching the nested env-var `anyOf`. Details and the control batch are in t002; the DoD gained a bullet for it.
