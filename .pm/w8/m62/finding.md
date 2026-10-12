# Blueprint validation accepts values that cannot reach the application environment

**Severity:** minor (confirmed false-positive validation; downstream production failure not exercised). **Attribution:** server semantic validation. **Source:** sweep171, 2026-10-12 UTC / 2026-10-11 America/Los_Angeles. **Status:** researched; fix scheduled, unimplemented.

Published Bex v0.3.2 and unmodified Render v2.27.0 at a764810a768202704e7206eb7b87a47211fcd98e both target `https://api.bex.co/v1/` with the isolated QA human OAuth grant in `bex-canary` (`tea-daif693dqjvc73e7as3g`). Source HEAD is 97ce481c8eabb13025c118ea9fa96d0aa83832d3; deployed revision is unknown. Commands use non-TTY JSON output.

## Reproduction and controls

Use the exact harmless [service literal](../evidence/m62/service-literal-nul.yaml), [service seed](../evidence/m62/service-seed-nul.yaml) and [group literal](../evidence/m62/group-literal-nul.yaml) fixtures. JSON is valid YAML; `\u0000` in these input files decodes to an actual NUL byte, rather than literal backslash text. Authenticate in an isolated CLI config and select the QA workspace.

```sh
bex blueprints validate service-literal-nul.yaml --confirm -o json
bex blueprints validate service-seed-nul.yaml --confirm -o json
bex blueprints validate group-literal-nul.yaml --confirm -o json
```

Repeat with unmodified same-pin `render`, explicitly targeting Bex and an isolated copied config; never use Render production. Then repeat the corresponding three `*-newline-control.yaml` files. [Full native receipts](../evidence/m62/native.json) preserve exact argv, stdout, stderr, exit and elapsed time for all 12 processes.

**Actual:** all six NUL checks return `valid:true`, print a plan, exit0 and emit no stderr. All six newline controls pass. **Expected:** NUL checks return the existing validation envelope with `valid:false` and field errors, producing native exit1. Newline controls stay valid.

The pinned consumer uses multipart `ownerId` plus the exact file bytes for `POST /v1/blueprints/validate`. It requires HTTP 200, prints the decoded body and uses `Valid` to select success versus exit1 (`cmd/blueprintvalidate.go:96–169`; generated `ValidateBlueprintResponse`). **Native wire traffic was not captured.** Exact documents, decoded responses and traced request construction are retained; they are not an intercepted HTTP record.

## Actual producer and consumer proof

The [actual compiler/parser diagnostic](../evidence/m62/parser.json) preserves all six values exactly in `req.Env`, `seedLiterals` or group `literals`. The [actual core patch diagnostic](../evidence/m62/core.json) accepts and preserves a JSON-decoded NUL value. Both diagnostics use virtual Go overlays, leaving product code unchanged.

The [local OCI control](../evidence/m62/local-env.json) uses the same pinned cached Python image for both inputs. Docker create succeeds for both. The ordinary 18-byte value starts and reaches the programme exactly. The NUL-containing 18-byte value fails at `docker start` with exit1 before the programme runs:

```text
invalid environment variable "QA_LITERAL": contains nul byte (\x00)
```

The [actual generated loader](../evidence/m62/generated-loader.sh.txt), executed unchanged in the same local image, decodes the 18-byte NUL value into a 17-byte value and continues with exit0. Bash reports `command substitution: ignored null byte in input`. The ordinary control remains exact. Full hashes, stdout, stderr, creation proofs and cleanup are in [local-loader.json](../evidence/m62/local-loader.json).

The [file control](../evidence/m62/local-file.json) reads the exact same 18-byte NUL payload from a mounted file successfully, with the original hash. This distinguishes environment values from arbitrary file bytes. POSIX excludes NUL from process environment values. [POSIX.1-2024 environment definition](https://pubs.opengroup.org/onlinepubs/9799919799/basedefs/V1_chap08.html).

These are local consumer executions. No NUL value was applied to a production App, no hosted runtime failure is claimed, and no Render-production behavior is claimed.

## Root cause and shared-code census

[Source excerpts](../evidence/m62/source.txt) and [counted callers](../evidence/m62/callers.json) retain producer and consumer boundaries.

- `apps/blueprint.go:657–661` compiles then parses a plan without checking process-value representability. `apps/deploy.go:463` preserves nonempty NUL strings correctly; `parseEnvGroup:2136` and `classifyServiceEnv:2376` check names but retain literal values. Each parser helper has one production caller; literals, seeds and groups are distinct branches.
- `core/environmentpatch.go:75–96` returns literal patch values unchecked. It has two production callers: `secrets/batch.go:538` and `envgroups/patch.go:187`. The actual core diagnostic covers this boundary; the live API routes were not called.
- `secrets/service.go:541 resolveValue` has three writer callers: full set, single set and seed. `secrets/files.go:480 checkCreateEnv` has two create-path callers. `envgroups/service.go:560 prepareCreateEnv` has two callers, including ApplyEnvGroup. These check presence, generation, names or quotas without the value constraint.
- `apps/service.go:2150 checkReservedSpecEnv`, `apps/deploy.go:1700 validateBlueprintEnvVarValues`, and environment quota validators need explicit caller verification when implementing the guard. Quota checks alone do not validate bytes.
- `operator/internal/build/native.go:235–242` puts decoded bytes into shell command substitution, where local execution loses NUL. `operator/internal/controller/app_controller.go:4740 appEnv` maps literal values directly into container env; `envFromSources:4771` projects group/service Secret values. These source consumers explain the proposed scope; production projection failure was not observed.

## Proposed fix and acceptance

Add one small environment-value guard in the backend core and apply it to newly supplied literals, Blueprint seed/group values, prompt overrides and every shared environment writer before effects. Reject NUL explicitly rather than silently stripping or encoding it into a different application value. Preserve current key rules, generation behavior and binary file content.

Use the existing Blueprint validation envelope (`valid:false`, field path/line where available). Ordinary writes should use existing bad-request mapping with a stable proposed code `ENVIRONMENT_VARIABLE_VALUE_INVALID`, identifying the key without its value. Confirm the corresponding GraphQL/MCP error mappings and dashboard feedback in the parity task. No CLI fork is needed.

Do not indiscriminately validate all already-stored map entries on every edit: users must be able to replace or delete legacy invalid values. Check new write values before persistence and before partial batch effects, while retaining existing repair paths.

Regression coverage must reject NUL at every position, preserve newline/Unicode/tab/CRLF/empty/numeric-string and literal backslash-u0000 controls, keep file NUL bytes exact, exercise both sparse-patch callers and the full-set/single/seed/create writers, and assert no partial write or deployment on rejection. Native Bex/Render validation must change from success to exit1 for the three invalid documents while all valid controls pass.

## Dedupe, limitations and cleanup

Open/done board searches found no NUL environment-value admission finding. m61 is about a representable PATH preventing helper lookup; 092 concerns explicit-empty presence; 095 concerns a missing empty group key; done/m53 covers numeric conversion; 027 covers key validation. This is a separate value constraint. Targeted source history does not show a NUL correction already landed. Reviewed `.pm/DO_NOT_DO.md`; this is core hosting configuration correctness.

Live direct environment writes, Blueprint apply, production NUL runtime, other runtimes/App kinds and full REST/GraphQL/MCP/dashboard behavior remain unverified. The Go probes diagnose current behavior and are not a full suite or an implemented regression test. All five local containers are confirmed absent; no hosting resource or group was created. The cached image is preserved. Filing is uncommitted.

**Estimate:** 155m across common validation, writer census and standing closing tasks.
