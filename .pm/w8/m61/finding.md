# A custom application PATH stops native build helpers before the user command

Why: accepted environment configuration prevents an otherwise executable absolute Python or Node build command from running. Adding another variable can turn a working PATH setting into a build failure.

Severity: **major**. Attribution: **operator/native build preparation**. Published Bex and the unmodified pinned Render client reproduce the hosted failure. The literal/Blueprint preparation variant is confirmed locally against generated container configuration; hosted Blueprint apply remains unverified.

## Reproduction and controls

Production `https://api.bex.co/v1/`, isolated QA human device grant, `bex-canary` / `tea-daif693dqjvc73e7as3g`, non-TTY JSON, 2026-10-12 UTC. Bex v0.3.2 / `a6350da0d6e451e063143432e06c548ba7b5851b`; unmodified Render v2.27.0 / `a764810a768202704e7206eb7b87a47211fcd98e`. Source diagnosis at `35e6d3b42b56d42fe031bc945748ebbbf91588bf`; deployed revision was not observable.

Both fresh fixtures are Free Python web services from the public bex repository, branch main, root `examples/hello-go`. The tested build and start commands call `/usr/local/bin/python3` by absolute path. The build records a harmless environment/file snapshot; the HTTP programme reads that snapshot and reports runtime hashes. Full programmes, inputs and argv are in [fixtures.json](../evidence/m61/fixtures.json) and [native.json](../evidence/m61/native.json). The abbreviated commands below preserve the relevant flags:

```sh
bex services create --name <fresh-qa-name> --type web_service --runtime python \
  --repo https://github.com/bex-co/bex --branch main --root-directory examples/hello-go \
  --build-command '<absolute Python programme from fixtures.json>' \
  --start-command '<absolute Python programme from fixtures.json>' \
  --health-check-path / --plan free --region frankfurt --auto-deploy=false \
  --env-var 'QA_COMPLEX=<harmless multiline marker>' \
  --env-var PATH=/usr/local/bin --secret-file qa-note.txt:<owned-local-file> \
  --confirm -o json
# Repeat through the unmodified same-pin Render client, with its own isolated
# equivalent config and RENDER_HOST=https://api.bex.co/v1/.
```

| Client / fixture | Native create | Bounded terminal observation | Path-only recovery |
| --- | --- | --- | --- |
| Bex / `srv-db62h3aktg8c739m65m0` | exit0, 1.515s, empty stderr | `build_failed`; public HTTP404 | native deploy exit0, 1.130s → Live/HTTP200 |
| Render / `srv-db62haskcuvc73d6ndag` | exit0, 1.695s, empty stderr | `build_failed`; public HTTP404 | native deploy exit0, 0.849s → Live/HTTP200 |

The recovery setup is an explicitly labeled same-authority owned API diagnostic, because the pinned CLI has no sparse env-save command:

```http
PATCH /v1/services/<owned-id>/environment
Content-Type: application/json

{"envVars":[{"key":"PATH","value":"/usr/local/bin:/usr/bin:/bin"}],"saveMode":"save_only"}
```

Both responses are HTTP200 with `rolledOut:false`. The follow-up commands are actual native `deploys create <owned-id> --commit 35e6d3b42b56d42fe031bc945748ebbbf91588bf --confirm -o json`. Both initial and recovered deploys resolve that same commit. Service readback confirms the build command is unchanged. Recovered HTTP responses report the exact new PATH at build and runtime, the original 30-byte environment marker and 24-byte file, with matching SHA256 values. [Complete wire diagnostics](../evidence/m61/wire.json), [runtime controls](../evidence/m61/runtime.json).

Native build-log reads exit0 in 0.575s / 0.456s and retain 100 records per client. The failed-deploy API details include the failure reason. Native JSON logs are a stream of objects; the first harness assumption that they formed an array was corrected, and is not a product finding. [Full native logs and selected failure messages](../evidence/m61/native-build-logs.json). Native create/deploy traffic was not intercepted; API reads and the path-only setup are separate diagnostics.

## Node hosted control — sweep166

A fresh owned Free Node web service, `qa-20261011-7b8b2e-nodepath` / `srv-db63a3dqamec73fj0s8g`, extends the mutable-input reproduction to another native runtime at source HEAD `a3129af44b6dcbabd88b2aa6a9d59ccfa544464c`. This journey used published Bex v0.3.2 only; the earlier Python reproduction remains the paired Bex/Render proof. [Complete commands, snapshots, wire setup and ownership](../evidence/m61/node-hosted.json), [tested driver](../evidence/m61/node-driver.py.txt).

1. Native create exits0 in 1.175s with **PATH alone**, `/usr/local/bin`, and absolute `/usr/local/bin/node` build/start commands. Deployment `dep-db63a3dqamec73fj0s90` becomes Live/HTTP200. Build and runtime report that exact narrow PATH and no QA_NOTE. This is a hosted positive control for the order-sensitive loader.
2. Owned API Save-only adds the harmless 17-byte QA_NOTE marker. HTTP200/`rolledOut:false`, unchanged deploy IDs and the original HTTP200 process show that this setup did not roll out. Native `deploys create srv-db63a3dqamec73fj0s8g --commit a3129af44b6dcbabd88b2aa6a9d59ccfa544464c --confirm -o json` exits0 in 0.824s, then `dep-db63bcskajsc73bvm3i0` becomes **build_failed**. The prior instance, built timestamp, narrow PATH and absent marker remain unchanged at public HTTP200; its deploy is still Live in the failure-stage history.
3. Owned API Save-only changes **only PATH** to `/usr/local/bin:/usr/bin:/bin`, again HTTP200/`rolledOut:false`. The same native pinned-commit deploy exits0 in 0.851s; `dep-db63d4icsrmc7381q1s0` becomes Live/HTTP200 on a new instance. Build/runtime agree on the exact complete PATH and marker length/hash. All three deployments resolve `a3129af44b6dcbabd88b2aa6a9d59ccfa544464c`; API readback confirms both commands stayed unchanged.

[Native build logs](../evidence/m61/node-native-build-logs.json) retain the full 100-object stream after the failure; failed-deploy detail preserves the exit1 reason. The codec hides its own stderr, and the build wrapper reports the command string, so the source/local mechanism controls above remain necessary to locate the failing helper. [Source identity](../evidence/m61/node-source-proof.json) confirms native.go is byte-identical to sweep163. Its default Node image is source metadata, not an observed deployed revision or runtime digest. No local generated-Node execution was added.

Cleanup: three owned deploy children enumerated; exact native service deletion exits0 in 1.810s. Detail/list/public absence, all six baseline service IDs and services6/Postgres0/KeyValue0 quota with termination0 are verified in the hosted artifact. No fixture survives. This extends the existing defect; it is not a second issue or an implemented correction.

## Actual generated-helper executions

A virtual Go test calls the unchanged `nativeDockerfile` and `nativeBuildPreparer` functions. Go1.26.5 diagnostic execution exits0; it extracts source artifacts, rather than asserting that the product is fixed. `native.go` is byte-identical between the preceding `55d03259b2` source extraction and the diagnosed HEAD. [Probe, command, source identities and limitations](../evidence/m61/source-execution.json).

The generated loader and RUN command were executed unchanged in the checkout's pinned Python image, `python:3.13-bookworm@sha256:073ffebb96ae4d0ed73ccad59f08c47bd84af8a79f5258130f8458ac50ad9ff4`, with only harmless mounted input records:

| Input record order | Result |
| --- | --- |
| Complete PATH, then QA_NOTE | exit0; absolute Python command runs and sees both values |
| `PATH=/usr/local/bin`, then QA_NOTE | exit1, empty stdout/stderr; Python command never runs |
| `PATH=/usr/local/bin` alone | exit0; Python command runs with that exact PATH |
| QA_NOTE, then `PATH=/usr/local/bin` | exit0; Python command sees both values |

The last row is a diagnostic order control, not a claim about actual preparation ordering. Actual Secret records use filename-glob ordering; literal keys are sorted. [Complete executions](../evidence/m61/local-loader.json).

The generated literal preparation container was also exercised using its pinned BusyBox image, `busybox:1.37.0@sha256:bdf57e528e45e4433820e045b29b4597825a1c9e38353532d90a01445013f82e`, its actual env/command/script, UID/GID65532 and a disposable `/native` tmpfs:

- Complete PATH succeeds, exit0.
- Literal `PATH=/usr/local/bin` cannot resolve the generated relative `sh` entrypoint, exit127.
- An explicitly diagnostic absolute `/bin/sh` control gets past startup but exits127 with `printenv`, `base64` and `tr` not found.

All seven local containers are absent. These local preparation observations are not hosted Blueprint or Kubernetes-controller acceptance. [Complete records](../evidence/m61/local-preparer.json), [generated artifacts](../evidence/m61/source-extraction.json).

## Producer and consumer mechanism

- The pinned `types.ParseEnvVar` uses `strings.Cut` and accepts the PATH string; `pkg/service/create.go:45,95,110–122` places the parsed key/value into `envVars`. The identical behavior in both clients excludes a Bex launcher workaround.
- `core.ReservedEnvKeys` contains only PORT (`core/config.go:147`); PATH is ordinary editable configuration, also recorded in w8/done/027. REST/GraphQL/MCP creates converge on the App create path. With the create seeder wired, literal create env becomes mutable env storage (`apps/service.go:2055,2135–2187`). Blueprint-owned literals stay on `spec.Env` (`:2297–2307`).
- Secret-backed values reach `projectNativeBuildEnv` (`operator/internal/controller/app_controller.go:1571`), then the native preparer writes the file records (`build/native.go:318–324`). The generated loader calls unqualified `base64` and exports each decoded variable immediately (`:235–242`). Exporting PATH changes command lookup before the next iteration, so the next decode exits before the user command in the generated RUN (`:249`). Its stderr is redirected away.
- Literal values enter the preparation container's env directly (`native.go:290–295`). A literal PATH therefore changes lookup for the relative `sh` command (`:313–315`) and the preparer's `printenv`, `base64`, `tr` and `mkdir` calls (`:322,329–333`) even before the generated RUN stage.

[Producer/consumer excerpts](../evidence/m61/source.txt). The two stages need one coherent correction: perform platform preparation with working platform tool lookup, then give the user command the exact configured PATH. Keep both mutable and literal sources, their precedence and the byte-preserving codec. A source shape that merely makes the shell path absolute still fails the codec control.

The existing [official CLI deployment guide](https://render.com/docs/your-first-deploy) documents that `--env-var` values are available during build and runtime. It does not certify this specific PATH experiment; authenticated Render-production behavior was not tested. Bex's own accepted PATH setting, local absolute-command controls and paired hosted recovery establish the functional defect independently. [Contract research](../evidence/m61/contract-research.json).

## Proposed fix and shared scope

Keep the preparer's own tool lookup usable instead of installing the user's PATH as its execution environment. Transport the user's literal PATH through the existing private preparation data, preserving its exact value. In the RUN loader, resolve/use the platform decoder independently of later PATH exports, or defer the final user PATH until all records have been decoded. Preserve last-wins ordering, empty/omitted distinction, Unicode, trailing newlines and equals signs. Do not append directories to the user's PATH, drop the variable, reserve it, rewrite CLI requests or introduce a second configuration writer.

Source census: **one production `nativeDockerfile` caller**, **one `nativeBuildPreparer` caller**, **one `projectNativeBuildInputs` caller**, **one `nativeBuildLiterals` caller**, and **two `buildEnv` callers**, including artifact identity. [Exact sites and family](../evidence/m61/callers.json). The shared native strategy covers elixir/go/node/python/ruby/rust, web/private/worker/cron, and static's native Node build. Dockerfile and kpack builders are separate. Postgres and Key Value do not use this App builder. This is a global native preparation fix. Mutable input was exercised on hosted Python and Node; generated literal preparation was exercised locally on BusyBox. The remaining runtime images and App kinds are still unverified.

Regression checks must execute meaningful helper behavior for mutable and literal PATH, later variables, PATH alone, omitted/empty PATH, full PATH and alternate input order. Retain literal-over-Secret precedence, image-default PATH, native env/file cache revision behavior and release identity. Verify that the exact custom PATH reaches the user build and runtime, rather than accepting a build that normalized it.

## Dedupe, cleanup and limits

All open/blocked/done workstreams were searched for custom PATH, loader/base64 lookup and native preparation. w6/done/m45 fixed the login-shell wrapper resetting image-default PATH; it does not isolate decoder tools from an explicit application PATH. w4/done/m92 fixed newline/Base64 transport, and w7/done/m87 versions the effective build inputs. w4/blocked/m163 adds file access; sweep162's byte controls and the same-file PATH-only recoveries pass that input class here. None owns this failure. Targeted `git log -S 'base64 -d'` still traces the unqualified decoder to source-build introduction; no newer implementation or deploy-lag correction was found. `.pm/DO_NOT_DO.md` and the compatibility ledger were reviewed.

Both hosted fixtures were deleted by recorded IDs. Each had two enumerated child deploys; detail/list/public absence passed, all six baseline services remain, and the final workspace quota is services6/Postgres0/KeyValue0 with termination0. Native deletes exit0 in 2.155s / 2.439s. [Ownership and cleanup](../evidence/m61/ownership-cleanup.json). Temporary Render configs were removed. The isolated QA grant remains for the requested loop.

Unverified: deployed revision, authenticated Render production, hosted Blueprint apply, Elixir/Go/Ruby/Rust and other App kinds, GraphQL/MCP/dashboard acceptance, replicas, Kubernetes descendants and a fixed product build. There is no product implementation, commit or push in this filing.

Estimate: **2h45m**, scheduled in six tasks including both preparation stages and shared-surface closing checks.
