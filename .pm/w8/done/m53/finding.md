# Numeric env values pass the Blueprint schema, then fail the typed decoder

Severity: **major** — a valid hosting configuration cannot be validated or planned, and source tracing shows the same failure precedes apply. Attribution: **backend**, independently reproduced in the released Bex launcher and the unmodified same-pin Render client.

## Context and reproduction

Production `https://api.bex.co/v1/`, QA human device grant, workspace `bex-canary` / `tea-daif693dqjvc73e7as3g`, non-TTY JSON. Tested 2026-10-09 UTC. Bex checksum-verified release v0.3.2; Render v2.27.0, commit `a764810a768202704e7206eb7b87a47211fcd98e`, module `v1.1.3-0.20260909214233-a764810a7682`. Source diagnosis at `30db645bf321627b083845528afd6e8265284c28`; deployed backend revision was not observable and remains unknown.

Save this as `numeric.yaml` (the fixture name is not a pre-existing resource):

```yaml
services:
  - type: web
    name: qa-20261008-e37b64-bp
    runtime: go
    repo: https://github.com/bex-co/bex
    rootDir: examples/hello-go
    plan: free
    buildCommand: go build -o app .
    startCommand: ./app
    envVars:
      - key: QA_MARKER
        value: 47
```

```sh
bex blueprints validate numeric.yaml --confirm -o json
# For the oracle, use an isolated same-pin Render config with the same QA grant,
# active workspace and RENDER_HOST=https://api.bex.co/v1/.
render blueprints validate numeric.yaml --confirm -o json
```

Expected: exit 0, `valid:true`, one planned service. Both actual clients exit **1** with empty stderr and this complete stdout:

```json
{
  "errors": [
    {
      "error": "decode compiled Blueprint: json: cannot unmarshal number into Go struct field bexEnvVar.services.envVars.value of type string"
    }
  ],
  "valid": false
}
```

Changing only `value: 47` to `value: "47"` makes both exit 0:

```json
{
  "plan": {
    "services": ["qa-20261008-e37b64-bp"],
    "totalActions": 1
  },
  "valid": true
}
```

| Input / location | Bex result | Control |
| --- | --- | --- |
| Root service: 47 / 47.5 / 1e3 | exit 1, same unlocated typed-decode error; 0.346 / 0.682 / 0.716s | exact integer quote-only control exit 0, 0.409s |
| Same-pin Render: 47 / 47.5 / 1e3 | exit 1, same error; 0.698 / 0.539 / 0.302s | integer quote-only control exit 0, 0.511s |
| Root env group value 47 | exit 1, 0.699s | string control valid, 2 actions, exit 0, 2.220s |
| Ungrouped service value 47 | exit 1, 0.636s | string control valid, exit 0, 0.987s |
| Project/environment service value 47 | exit 1, 0.307s | string control valid, exit 0, 0.900s |
| Project/environment env group value 47 | exit 1, 0.967s | string control valid, 2 actions, exit 0, 1.242s |

The five location errors differ only in the Go struct-field path. Full commands, stdout, stderr, durations and manifests are saved in [CLI evidence](evidence/cli-repro.json) and [container evidence](evidence/containers.json). No failed transport, throttling, auth refusal or async resource convergence explains this deterministic input failure.

## Contract and mechanism

- The pinned `internal/apps/schema/render.yaml.json:423–424` explicitly allows `string` or `number` for `envVarFromKeyValue.value`. That definition also supplies group vars. `schema/capabilities.json:327–331` classifies the field as translated to a literal/seed. The current [official Render schema](https://render.com/schema/render.yaml.json) was checked independently and has the same union; it is corroboration, not a replacement for the repository pin. Authenticated Render-production validation and exact Render number-to-process formatting were not tested.
- Producer: `blueprint_compiler.go:1026–1066` retains a numeric YAML scalar as int64 or float64, then the schema and capability checks accept it. `CompileBlueprintIR` (`blueprint_ir.go:213`) produces the compiled source/IR. All five private probe fixtures pass this real compiler with zero problems.
- Consumer: `bexEnvVar.Value` is still a **string** (`deploy.go:446–449`). `parseCompiledStack:1786–1791` calls `decodeCompiledBlueprintManifest:2051–2063`, which JSON-marshals the compiled AST and unmarshals its numbers directly into that string field. The failure is before container flattening or env classification, so nested forms and groups share the defect.
- `blueprintValidationFor` (`blueprint.go:657–661`) uses that adapter after compiler success. Its generic bad-request fallback (`:743`) has no matching resource name/location in the decoder text; hence REST omits path/line/column and GraphQL returns nulls. Validating strings succeeds through the identical route and grant.
- Eventual process consumers already require strings: `classifyServiceEnv` (`deploy.go:2339–2400`), env-group parsing (`:2102`), sibling `envVarKey` copies and `ApplyBlueprintServiceSpec`'s env merge (`blueprint_plan.go:254–256`). Fix this typed boundary once; leave every unrelated numeric field typed.

The pinned CLI constructs multipart `ownerId` plus `file` without converting YAML values (`cmd/blueprintvalidate.go:97–125`), decodes the HTTP 200 validation result and correctly exits 1 on `valid:false` (`:144–161`). There is no launcher or upstream request defect to work around.

## Wire and source evidence

An equivalent multipart replay with the unchanged file, `POST /v1/blueprints/validate`, `ownerId=tea-daif693dqjvc73e7as3g`, and actual client UA `render-cli/2.27.0 (macOS - 26.5.1)` returned HTTP **200**, `Content-Type: application/json`, `cf-ray: a47b7f6fd827cf1b-SJC`, and the complete same failure JSON. Authorization is omitted. This is an explicitly labeled replay, **not intercepted CLI traffic**; the MIME boundary can differ. [Complete non-secret request and response](evidence/wire.json).

The API-only GraphQL diagnostic `validateBlueprint(ownerId:$owner,bexYaml:$manifest)` returns HTTP 200, `valid:false`, the same errors and `errorDetails:[{column:null,line:null,path:null}]`. Its quote-only control returns `valid:true`, `errors:[]`, `errorDetails:[]`. [Exact GraphQL queries, variables and responses](evidence/graphql.json). These are diagnostic API checks, not extra CLI coverage.

A diagnostic test in a private source copy invokes the actual `CompileBlueprintIR` followed by `parseCompiledStack`. The five source files governing the producer/consumer/schema were byte-compared with the diagnosed HEAD and match. `GOWORK=off GOTOOLCHAIN=go1.26.5 go test ./internal/apps -run '^TestQANumericEnvLiteralAccepted$' -count=1` fails all five cases with the same typed-decode error, **exit 1, 14.558s**. This expected failing probe establishes the mechanism; it is not a fixed regression suite. [Self-contained probe](evidence/qa_numeric_env_probe_test.go.txt), [full probe result](evidence/source-probe.json). Product source was not changed.

## Proposed correction and verification

Accept numeric literals at the typed env-value boundary and convert them once to canonical JSON-number text. A narrowly scoped decoder for `bexEnvVar` can retain the existing string model while decoding the value as string-or-number; use raw JSON/json.Number rather than routing integers through float64. Preserve quoted strings byte-for-byte, omission/empty handling and seed/link/reference rules. Do not stringify disk sizes, ports, plans or the whole AST; do not reparse the original YAML, weaken schema validation, or teach the CLI to quote values.

The proposed canonical results are `47`, `47.5`, `1000` for 47/47.5/1e3. Retain exact supported integer digits; the compiler's existing int64/finite-float grammar remains the boundary. Unsupported scalar forms continue to get source-located compiler/schema errors, rather than Go decoder internals. `previewValue` remains an explicit non-goal.

Verify both schema acceptance and typed/runtime strings, all containers, group literals, sync:false seeds, numeric zero, sibling envVarKey copies, and a second unchanged sync with no env-induced diff/deploy. Preserve string whitespace/leading zeroes and existing generated/omitted/empty behavior. Boolean/null/array/object refusals retain the field location. Those neighboring cases are scheduled acceptance work, not claims of extra live failures.

## Shared-code census and limits

There are **four direct production parseCompiledStack call sites**: compileStack (`deploy.go:1659`), validate/preview (`blueprint.go:661`), disconnect grouping reclamation (`:1607`) and generated-manifest self-validation (`blueprint_generate.go:240`). `parseStack` is a test helper forwarding into compileStack, not another live API entry point. compileStack feeds direct DeployStack, CreateBlueprint, SyncBlueprint and sync's second prepared-apply pass (`deploy.go:635`; `blueprint.go:1034,1225,1407`).

REST validates at `rest.go:1331–1352`; GraphQL `validateBlueprint` at `graphql.go:1251–1262`; MCP `validate_bex_yml` at `mcp.go:1019–1025`. Their preview/create/sync/direct-deploy counterparts converge on the same roots. The dashboard hook `dashboard/src/features/blueprints/hooks/use-validate-blueprint.ts:26` consumes the GraphQL result; UI behavior was not driven this sweep.

Affected declarations: the five App kinds' env vars and env groups, at root/ungrouped/project-environment locations. Live coverage here is web plus env groups; worker/private/cron/static are source-predicted. Postgres/Key Value declarations do not themselves carry this literal field, although an entire mixed stack is stopped by the failed env decode. Generated exports normally emit strings, so that caller is a regression control, not a claim that exports currently fail. Disconnect cleanup, apply/runtime, MCP and dashboard outcomes were not exercised live for this finding.

## Dedupe, boundaries and cleanup

Searched all open/blocked/done `.pm` files for the complete error, decodeCompiledBlueprintManifest, bexEnvVar and numeric/env/value combinations, and inspected the related history. w4/done/m102 repaired schema anyOf error selection, not this post-schema adapter; w4/blocked/m157/m158 concern SQL result numbers; w8/done/m48 concerns validation upload size; w4/done/m147 concerns omitted/empty env application. None owns numeric Blueprint literals. At the original diagnosed HEAD, the string-only field and unmodified decoder remained; no newer implementation or deploy-lag correction was found then. The later source fix and live recheck are recorded below. `.pm/DO_NOT_DO.md` and the CLI compatibility ledger were checked. The existing schema/capability promise makes this in scope without changing a non-goal.

All tests in this finding are validation/read-only diagnostics. No hosting resources, projects, environments or groups were created. The copied isolated Render config was removed without logging out the ongoing Bex grant; no personal CLI state was used. Prior sweeps' fixture cleanup is recorded separately. The QA auth session remains active for the requested loop. Filing only: no product implementation, commit or push was performed by this hunt.

## Sweep 29 production recheck after the source fix

Main now includes `1608b22e83dfe9344e7e98a850dd8b61274dad59`, authored outside this QA hunt. The published Bex v0.3.2 still rejects integer, float and scientific numeric fixtures at the compiled-to-typed decoder; the quoted integer control remains valid, and the boolean control remains a located schema refusal. This is the existing finding, with no new resource mutation or duplicate filing. Production revision was not observable, so neither successful deployment nor a defect in the new implementation is claimed. See [the sanitized live recheck](evidence/live-after-source-fix.json); t003 remains pending.

## Sweep 38 independent production validation recheck

After main's image-pin advance and the separately authored milestone closeout, published Bex v0.3.2 and the unmodified Render v2.27.0 pin both validate unchanged integer `47`, float `47.5`, scientific `1e3`, and quoted integer manifests with exit 0 and `valid:true`. The boolean control remains exit 1 with `valid:false` and the expected `services[0].envVars[0].value` location (line 12, column 16). All ten fresh command processes passed. See [the complete sanitized recheck](evidence/production-recheck-after-rollout.json).

This independently verifies the original root-service validation failure after rollout configuration changed. Actual production revision remains unobserved. This validation-only sweep did not run apply, process-value checks, repeated unchanged sync, every nested container, MCP, or dashboard acceptance; the milestone's external closeout evidence remains separately attributed. No hosting resources were created. The copied Render config was removed without revoking the ongoing isolated Bex grant.
