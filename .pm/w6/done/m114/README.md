# w6 · m114 — Generate Blueprint emits the tenant-prefixed CR name, so bex's own exporter produces a `render.yaml` bex's own validator rejects

**Worker:** worker6 **Goal:** the file Generate Blueprint hands you is a file you can commit and apply — carrying each service's public name, not the internal object name that both invalidates the manifest and writes the workspace's tenant id into a repo. **Status:** done (2026-08-26)

## Resolution (2026-08-26)

Triaged, confirmed real, and fixed. The report reproduces **byte for byte** in a focused unit test: with `"name": a.Name`, `GenerateBlueprint` emits `name: tea-d98210cbbpdc73dcrkvg-qa-20260826-webhook-svc` and `ValidateBlueprint` returns `valid: false` with the exact `name must be a DNS label of 1-30 chars ([a-z0-9-])` error; swapping in the public name flips it to `valid: true`.

**Why the generator's self-check missed it** (the report's open question about the closed loop): `GenerateBlueprint`'s self-check (`blueprint_generate.go:162-170`) runs only `CompileBlueprintIR` + `parseCompiledStack`. The 30-char `ValidAppName` gate lives in `specFromCreate` (`service.go:2166`), reached by `ValidateBlueprint` → `validateBlueprintServices` (`deploy.go:1185`) but **not** by the parser — so a 48-char name passed generation yet failed validation. That gap is why the bad file reached users; recorded here rather than widened, because pulling the full create boundary (with its maintenance-mode/URL-ownership checks) into an *export* path would wrongly reject exporting a service in a maintenance window.

**Fix** — one line at the single producer, `blueprint_generate.go:177`: `"name": a.Name` → `"name": appServiceName(a)`. `appServiceName` (`blueprint_ownership.go:165`, already in-package, doc-labeled "the manifest-facing service name") reads `LabelServiceName` (the public name), falling back to `a.Name` only for a legacy hand-applied App with no such label — exactly the target behavior, including the legacy edge case. The datastore siblings already emitted `Spec.Name`, so this aligns services with them and leaves Postgres/KeyValue output byte-unchanged.

**Caller count (DoD #6):** the generate verb has **three** surfaces — REST `POST /v1/blueprints/generate`, GraphQL `Query.generateBlueprint`, MCP `generate_blueprint` — all routing through the single `s.GenerateBlueprint` → `generateServiceEntry` producer, so the one-line fix covers every surface at once. No per-surface divergence.

**Regression test:** `TestGenerateBlueprintServiceNameIsPublicNotCRName` (`blueprint_generate_test.go`) — the first fixture to model a store-managed (tenant-prefixed) App; asserts the manifest carries the public name, contains no `tea-` tenant id, does not carry the CR object name, and self-validates through `ValidateBlueprint`. Proven to fail without the fix.

**Scope note vs. the original 7-task plan:** the substance (the defect, its class explanation, the caller count, the regression guard) is resolved by the change above. The fix is kind-agnostic — `generateServiceEntry` names all five App kinds uniformly, and the existing `TestGenerateBlueprintDomainsCronAndWorkerScaling` already self-validates web/cron/worker output. No render-parity ADR edit is needed: the change fixes a producer bug against the *existing* `render.yaml` contract (public name), it does not alter the contract. Checks run: `gofmt`, `go build ./internal/apps/`, `go vet ./internal/apps/`, and `go test ./internal/apps/ -run TestGenerateBlueprint` (green).

## Background (found live, 2026-08-27, 26th `/qa-find-bugs` run)

Journey 13. On `https://dashboard.bex.co/blueprints`, pressed **Generate Blueprint**, selected one service (`qa-20260826-webhook-renamed`, `srv-da7o6ovvqdcc73bpn9hg`), and pressed **Generate**. The dialog — whose own copy says *"Select existing resources to export as a render.yaml you can commit to a repo and connect as a Blueprint"* — returned:

```yaml
services:
  - buildCommand: go build -o app .
    envVars:
      - key: QA_TEST_VAR
        sync: false
    name: tea-d98210cbbpdc73dcrkvg-qa-20260826-webhook-svc
    repo: https://github.com/bex-co/bex-hello-go-live.git
    runtime: go
    startCommand: ./app
    type: web
```

`name` is the **Kubernetes object name** — `core.CRName(tenantID, name)` = `<tenant>-<public name>` — not the service's public name (`qa-20260826-webhook-svc`) or its display name (`qa-20260826-webhook-renamed`).

### The loop is closed, and it fails

Fed that exact output straight back to bex's own validator, in-page (`fetch(..., {credentials:'include'})`, per this hunt's Phase-3 trap about bare-UA clients getting Cloudflare `1010`):

```
query V($bexYaml: String!) { validateBlueprint(bexYaml: $bexYaml) { valid errors } }

# the generated file, byte for byte:
=> { "valid": false,
     "errors": ["service \"tea-d98210cbbpdc73dcrkvg-qa-20260826-webhook-svc\": bad request:
                 name must be a DNS label of 1-30 chars ([a-z0-9-])"] }

# the identical file with ONLY the name replaced by the public name:
=> { "valid": true, "errors": [] }
```

The generated name is **48** characters against `ValidAppName`'s 30-char cap (`lego/backend/internal/store/api.go:588`, `^[a-z0-9]([a-z0-9-]{0,28}[a-z0-9])?$`). The tenant prefix alone is 25 characters (`tea-` + a 20-char id + `-`), so **any service whose public name is 6 characters or longer** — which is essentially all of them — exports a manifest that cannot be applied. Export → import is broken by construction, not by edge case.

The single-field control is what makes this airtight: same YAML, same validator, one field changed, `false` → `true`.

### It also writes the tenant id into a file you are told to commit

`tea-d98210cbbpdc73dcrkvg` is the workspace's internal identifier. The same dialog reassures the user about hygiene in the panel directly below the output — *"Secret values are never exported — secret-backed variables appear as `sync: false`"* — while emitting the tenant id in plain text on every service line. Not a credential, but an internal identifier the product otherwise keeps out of user-facing surfaces, landing in a file whose stated purpose is to be committed to a repo.

## Root cause

`lego/backend/internal/apps/blueprint_generate.go:174-179`:

```go
func (s *Service) generateServiceEntry(ctx context.Context, a *appv1alpha1.App, dbDisplayByID, kvDisplayByID map[string]string) (map[string]any, error) {
	svcType := effectiveType(a.Spec.Type)
	entry := map[string]any{
		"name": a.Name,          // ← the CR object name, tenant-prefixed
		"type": blueprintTypeSpelling[svcType],
	}
```

`a` is an `*appv1alpha1.App`, so `a.Name` is `core.CRName(tenantID, name)` (`lego/backend/internal/core/base.go:1379`, applied at `lego/backend/internal/apps/service.go:1736`).

**The sibling paths in the same file are correct, and that is the control:**

| resource | line | emits | correct? |
| --- | --- | --- | --- |
| service | `blueprint_generate.go:177` | `a.Name` (CR object name) | **no** |
| database | `blueprint_generate.go:361` | `d.Spec.Name` | yes |
| key value | `blueprint_generate.go:397` | `kv.Spec.Name` | yes |
| `fromService` / `fromDatabase` cross-refs | `:328`, `:347` | a resolved `display` name | yes |

Verified live, not just read: generating from a database alone returns

```yaml
databases:
  - name: beancount-forum-db
    plan: basic-256mb
```

— the bare public name, no prefix. So datastores are fine *for the reason claimed* (they read `Spec.Name`), and the defect is confined to the service entry.

**The layer can express the fix.** `core.LabelServiceName` (`lego/backend/internal/core/base.go:175-182`) is documented as carrying "an App CR's **PUBLIC** name (w4/m19)" and is set on every App the generator already holds. The edge case to handle: a legacy bare-object-named App (created before the w4/m19 scheme, documented at `core/base.go:1381-1393` as never renamed in place) carries no such label, and `tenantID == ""` skips the prefix entirely — for those, `a.Name` already *is* the public name.

## This is a known defect class landing on a new surface

`w4/m19` (done 2026-07-14) found and fixed exactly this shape, and its own summary names it:

> "a real name-leak bug — **the create response echoing the tenant-prefixed CR name instead of the public service name** — found and fixed in the t009 simplify pass, **with a regression test**"

that test being `TestCreate_ResponseNameIsThePublicNameNotTheCRName` (`createowner_test.go`, per `w4/m19/t010`). **m19's fix still holds** — `GET /v1/services/srv-da7o6ovvqdcc73bpn9hg` returns `"name": "qa-20260826-webhook-renamed"` with `"slug": "qa-20260826-webhook-svc"`, the public pair. So this is **not** a regression of m19. It is the same class reappearing on a surface that did not exist when m19's regression test was written: Generate Blueprint shipped later, in `w8/m22` (`487eab31`, "feat(blueprints): Generate Blueprint — export existing resources as render.yaml"). A per-call-site regression test could not cover a call site added afterwards, which is why t003 asks for a guard over the class rather than one more per-site assertion.

## Target behavior

The exported `name` is the value a user would type to create the same service — the public name (`LabelServiceName`), falling back to `a.Name` only for the legacy unprefixed case. Concretely: **every manifest Generate Blueprint produces must satisfy `validateBlueprint` with `valid: true`**, and must contain no `tea-…` tenant identifier anywhere.

## Blast radius

`generateServiceEntry` is the single producer of service entries; the whole feature reaches users through `GenerateBlueprint` (`blueprint_generate.go`), surfaced in the dashboard's Generate Blueprint dialog. Whether REST/GraphQL/MCP each expose the generate verb — and therefore how many entrypoints carry the bug — was **not** enumerated this run; t001 must grep and give the count rather than assume the dashboard is the only caller.

The five App-typed kinds (web · private · worker · cron · static) all flow through `generateServiceEntry`, so all five export an invalid name; only `web` was exercised live. Postgres and Key Value are unaffected, verified above.

## Adjacent classes

Not an authorization boundary. The distinction that matters is public-name vs object-name vs display-name — three values that a service legitimately has at once (`name` / `slug` / `displayName`, all three visible on `GET /v1/services/<id>`). t001 must say which of the three the manifest carries and why: `displayName` is mutable and non-unique, so the public **name** is the only one that can round-trip through create.

## Unverified (carried forward)

- Whether the exported manifest is valid **once the name is fixed** for kinds other than `web`, and for services with disks, cron schedules, static publish paths, or health-check paths — the name is the first error the validator returns, and fixing it may reveal others behind it.
- Whether REST/GraphQL/MCP expose the generate verb, and how many callers exist.
- Whether the `sync: false` env-var round-trip actually works on apply — the dialog promises the values are "prompted for when the Blueprint is first created", which this run did not exercise.
- Whether any other field in the generated output carries an internal identifier; only `name` was inspected closely.

## Tasks (in order)

| id   | title                                                                                | est | depends_on |
| ---- | ------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Export the public service name, and count the generate verb's callers                  | 40m | —          |
| t002 | Round-trip the generated manifest through `validateBlueprint` for every service kind    | 45m | t001       |
| t003 | Guard the class: no bex-facing surface emits a CR object name where a public name belongs | 40m | t001       |
| t004 | Render parity — the generate verb and the manifest's name field across surfaces          | 30m | t002, t003 |
| t005 | Simplify — `/simplify` over the code this milestone changed                              | 20m | t004       |
| t006 | Test coverage                                                                            | 40m | t004       |
| t007 | Closeout                                                                                 | 15m | t006       |

## Definition of done

1. Generate a blueprint from any one service on `https://dashboard.bex.co/blueprints`; feed the output verbatim to `query { validateBlueprint(bexYaml: "<output>") { valid errors } }` and get `valid: true, errors: []`. Today the same two steps return the 1-30-chars error, and the single-field control (swapping in the public name) already returns `valid: true`.
2. The generated manifest contains no `tea-` tenant identifier — `grep -c 'tea-' <output>` is `0`.
3. The exported `name` equals the service's public `name` from `GET /v1/services/<id>` (`qa-20260826-webhook-svc` for the fixture), not its `displayName` and not its CR object name.
4. Databases and Key Value entries are byte-unchanged: generating from `beancount-forum-db` alone still yields exactly `databases:\n  - name: beancount-forum-db\n    plan: basic-256mb` — they are correct today and must stay correct.
5. Bullet 1 holds for a second service kind beyond `web` (per t002's enumeration), or t002 records which kinds still fail and why, as work rather than as a silent gap.
6. t001's caller count for the generate verb is recorded in this README.
7. `w4/m19`'s own guarantee still holds: the create/read response still returns the public name pair (`name` + `slug`), verified rather than assumed.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 26th run, 2026-08-27, journey 13 (Blueprints). Every probe and its complete response is pasted inline — for a contract the durable artifact is the request and the response, and both the failing and the control validation are re-runnable by anyone. Nothing was created or mutated this run: Generate Blueprint is read-only, so there is no cleanup. `.playwright-mcp/` captures are gitignored and nothing here rests on them.
- **Goal linkage:** `docs/ADR049-render-yaml-parity.md` owns the `render.yaml` contract and `docs/ADR018-render-parity.md` the parity ledger; `w8/m22` shipped this exporter. ADR008's hosting pillar — an IaC export that its own importer rejects is worse than no export, because the failure surfaces only after the user has committed the file.
- **Expected outcome:** Generate Blueprint becomes a real migration path — export, commit, connect — instead of producing a file that fails at the first apply, and stops writing the workspace's internal id into user repositories.
- **Why now:** the defect is total (every service, not an edge case) and silent at generation time — the dialog offers **Copy** and **Download render.yaml** with no validation gate, so the user finds out only when the Blueprint they committed fails to sync. It is also a class the codebase already paid to fix once in `w4/m19` and re-shipped on a newer surface, which is the argument for t003's guard rather than a third one-line fix.
- **Render parity task included:** yes — the change alters the manifest contract the `render.yaml` parity ADR owns and the output a dashboard surface hands users; t004 also settles whether REST/GraphQL/MCP expose the verb at all.
