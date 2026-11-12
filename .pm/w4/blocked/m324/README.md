# w4 · m324 — A native-runtime build log is BuildKit's transcript of bex's own Dockerfile: 132 of 187 lines are `#N` steps, layer digests and platform internals, and the build command's output arrives prefixed `#23 2.411`

**Worker:** worker4 **Goal:** a Node, Python, Ruby, Go, Rust or Elixir build's log reads like Render's. It narrates the build command, shows that command's output as the command printed it, and says when the build succeeded or failed. bex's internal build recipe stays out of the tenant's log. Docker-runtime builds keep BuildKit's output, as on Render. **Status:** blocked — t001–t003 and t005–t007 done (shipped); t004 live replay waits for the next production deploy, then t008 closes

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | A native build's log shows the build command and its output, not BuildKit's steps for bex's generated Dockerfile — **DONE** | 90m | — |
| [t002](done/t002.md) | The failure reason and the `==> Build failed:` narration quote the command's output without BuildKit's elapsed-time prefixes (residual of w4/485) — **DONE** | 20m | — |
| [t003](done/t003.md) | Blast radius: every native runtime and service type, the Docker and buildpack paths, live tail and stored logs — **DONE** | 20m | t001, t002 |
| [t009](t009.md) | The live replay's failures: a failed build command's `failureReason` and `==> Build failed:` quote bex's toolchain steps as the "build environment", and push digests reach the log | 60m | t003 |
| [t004](t004.md) | Live verification in `bex-canary` | 30m | t003, t009 |
| [t005](done/t005.md) | Render parity — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 10m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | t005 |
| [t008](t008.md) | Closeout | 10m | t006, t007, t009 |

Nine tasks, 285m (4h45m).

**Minor (parity and diagnosability, on every native build).**
- The build log is where a user reads why a build failed or what it installed. On a native build it's mostly bex's own container recipe.
- That recipe is a static-ffmpeg image layer, a pnpm tarball `ADD`, `/opt/bex/load-env` and `RUN --mount=type=secret,id=render-env,…`.
- The user's own output starts 45 lines down.

## Finding

Live `/qa-find-bugs w4` loop, round 84, 2026-11-11 07:47–07:55 UTC (`muse.env`, workspace `bex-canary`; production runs the code of `348bd77a5`).

Fixture: Free web service `qa-20261111-bf84` (`srv-dbq1rdik10fs73dvp2b0`), created in the dashboard from Public Git URL `https://github.com/render-examples/express-hello-world` (Node runtime). It was deleted afterwards (DELETE 204, then GET 404).

### Probes

All API reads ran from the signed-in dashboard page with `fetch(…, {credentials: "include"})`. Logs were paged with `GET /v1/logs?ownerId=tea-daif693dqjvc73e7as3g&resource=srv-dbq1rdik10fs73dvp2b0&type=build&direction=forward&limit=100&startTime=…&endTime=…`, following `nextStartTime` while `hasMore`.

- **Failed build.** Build command `npm install && echo QA84 failing on purpose && exit 3`; deploy `dep-dbq1rdik10fs73dvp2bg`.
  - The log has 179 lines: 122 start with a BuildKit `#N` step number, and 58 contain `sha256:`.
  - The only `==>` narration is `Build queued`, `Using Node 24 (default)`, `Building from https://github.com/render-examples/express-hello-world@039c347`, and the failure block.
  - The first line naming npm or the build command is line 45: `#11 [stage-0  4/12] ADD --link --checksum=sha256:c729… https://registry.npmjs.org/pnpm/-/pnpm-10.34.7.tgz /opt/bex/pnpm.tgz`.
  - **Platform internals in the log:**
    - `#6 [internal] load metadata for docker.io/mwader/static-ffmpeg:9.0.2-1@sha256:f5d1…`;
    - `#4 [context bex-init] load .dockerignore`;
    - `#19 [stage-0  7/12] RUN ln -sf /ffmpeg /usr/local/bin/ffmpeg && ln -sf /ffprobe /usr/local/bin/ffprobe`;
    - `#22 [stage-0 10/12] COPY <<BEX_LOAD_ENV /opt/bex/load-env`;
    - `#23 [stage-0 11/12] RUN --mount=type=secret,id=render-env,target=/run/secrets/render-env ["/bin/bash","-c",". /opt/bex/load-env\nnpm install…`.
  - **The npm output** reads `#23 2.344 6 vulnerabilities (1 low, 2 moderate, 2 high, 1 critical)`, and so on.
  - **`failureReason`** (REST deploy, `deploy_ended` details, the deploy page and the Deploys row agree): `build failed in the docker build step:\n2.331 13 packages are looking for funding\n2.331   run \`npm fund\` for details\n2.344 \n2.344 6 vulnerabilities … \n2.413 QA84 failing on purpose\nerror: build command 'npm install && echo QA84 failing on purpose && exit 3' exited with code 3`.
  - **The log's failure narration** repeats that string one `==>` line per line, so it reads `==> 2.344 6 vulnerabilities …` and `==> 2.413 QA84 failing on purpose`.
- **Successful build.** After a PATCH of `buildCommand` to `npm install`, deploy `dep-dbq1tfqp92ls73cjjqfg` went `live`, and `curl https://qa-20261111-bf84.onbex.co/` returned 200.
  - Its log has 187 lines: 132 `#N` lines and 81 with `sha256:`.
  - The narration is only `==> Build queued`, `==> Building from …@039c347`, `==> Using Node 24 (default)` and `==> Your service is live 🎉`.
  - The build command's output is twelve lines, each prefixed `#23 2.411 ` or `#23 2.424 `, for example `#23 2.411 added 80 packages, and audited 81 packages in 2s`.
  - There is no `==> Running build command 'npm install'…` and no `==> Build successful 🎉`.

### Root cause

- **The native path runs BuildKit on a generated Dockerfile.** `nativeDockerfile` (`lego/operator/internal/build/native.go:310-345`) writes a multi-stage Dockerfile: the toolchain base, the static-ffmpeg and pnpm tool layers (`nativeMediaTools`, `nativeNodeTools`), `COPY . .`, the env-loader heredoc, and one `RUN --mount=type=secret,id=render-env,… <loader> + <buildCommand>`.
- **Its full transcript is the log.** `buildctl-daemonless.sh` runs with no `--progress` flag (`lego/operator/internal/build/build.go:1063-1083`), so BuildKit prints `--progress=plain` for a non-tty. The build pod's stdout is shipped verbatim to the `build` log type (`deploy/gitops/base/log-shipper.yaml`, `build_pods` pipeline; ADR010 § build). Nothing filters it for native runtimes.
- **The progress-line contract assumed the noise was useful.** `docs/render-artifacts/live-deploy-following.md` § Platform progress-line contract says clone and checkout "arrive as real BuildKit stdout once the pod runs" and that bex needs "a compatible _reading experience_". w8/052 explicitly skipped the optional `==> Running build command '<cmd>'…` narration.
- **The failure summary keeps BuildKit's timestamps.** `buildkitFailureSummary` (`build.go:815-839`) keeps the `------` block's body lines verbatim, including each line's elapsed-seconds prefix. w4/485's expected shape was the command's output alone (`compiling`, not `0.087 compiling`).

### Fix (target behavior)

- **t001: the native build log.** For native runtimes only, the build log carries:
  - bex's existing `==>` narration;
  - a `==> Running build command '<cmd>'…` line, with the user's command as configured, not bex's loader;
  - the build command's own stdout and stderr, one line per line it printed, without `#N` or elapsed-seconds prefixes;
  - `==> Build successful 🎉` or the existing failure line.

  BuildKit's bookkeeping for the generated recipe stays out of the tenant log: `FROM`/`resolve`/`sha256`/`extracting`/`DONE`, the tool layers, `COPY`, and the loader and secret-mount step header.
  - **Mechanism, decide and record:** run buildctl with `--progress=rawjson` and translate in the build container, keeping only the vertex of the user's `RUN` step (`buildkitNativeRunStep`, `build.go:772`, already identifies it); or filter `plain` output by step number.
  - **Docker-runtime builds** (a user Dockerfile) keep BuildKit's plain output unchanged, as Render shows it.
  - **Errors stay visible.** A BuildKit error outside the user's step (a toolchain pull, a cache import) still reaches the log as its own `==>` line naming the phase, never silently dropped (w6/m123's guarantee).
- **t002: the failure reason.** The failure reason and the `==> Build failed:` lines strip the `^\d+\.\d+ ` elapsed prefix from the quoted command output. A Docker-runtime failure reason may keep BuildKit's text. Then decide whether "in the docker build step" is the right phrase for a native runtime (for example "while running the build command").

### Blast radius

- **Native runtimes:** `nativeRuntime(o)` covers Node, Python, Ruby, Go, Rust and Elixir, plus static-site native builds. Service types are web, private, worker, cron and static.
- **The live tail** (`GET /v1/logs/subscribe` follows `PodLogStream`, ADR010:26) and stored Loki logs must read the same filtered stream.
- **The buildpack (kpack) path** (`build/kpack.go`) and pre-deploy logs are separate producers; t003 records whether they change.
- **The failure-reason consumers** read one string: deploy row, deploy page, Events, GraphQL, MCP and the failure email.

### Unverified

- Python, Ruby, Go, Rust, Elixir and static-site native builds. Only Node was probed; they share `nativeDockerfile` by reading.
- The failure email body, assumed to reuse `failureReason`.
- Render's exact current native-log lines. The canonical vocabulary comes from `docs/render-artifacts/live-deploy-following.md`, which notes it couldn't be captured live.

### Render

- Render's native build log narrates `==> Running build command '<cmd>'…`, then prints the command's output, then `==> Build successful 🎉` (docs/render-artifacts/live-deploy-following.md, render.com/docs/your-first-deploy).
- Docker builds show BuildKit output.

### Dedupe

- **w4/485 (done)** trimmed the failure reason's BuildKit bookkeeping; t002 is its residual.
- **w6/m123 (done)** made BuildKit's output reach the log at all.
- **w8/052 (done)** rewrote the wrapper in failure summaries and skipped the narration.
- **w8/069 (blocked)** covers the `==> Using <runtime>` line sometimes missing.
- `grep -rli "static-ffmpeg\|bex-init\|Running build command" .pm` finds no item about the native log's content.
- **Not deploy lag:** `git log 348bd77a5..origin/main -- lego/operator/internal/build` shows only `b8e6c3c2c` (w8/m154, secret-file links), which does not touch progress output.

## Definition of done

Signed in to `bex-canary` after the release pins this build, on an owned Free Node web service from `https://github.com/render-examples/express-hello-world`, deleted afterwards:

1. **A successful build reads like Render's.** With build command `npm install`, the deploy's build log (paged `GET /v1/logs?…&type=build`) shows:
   - `==> Running build command 'npm install'…`;
   - npm's own lines with no `#N` or `d.ddd` prefix (for example `added 80 packages, and audited 81 packages in 2s`);
   - `==> Build successful 🎉`.

   No line contains `sha256:`, `static-ffmpeg`, `bex-init`, `/opt/bex/load-env` or `--mount=type=secret`. The dashboard deploy page shows the same.
2. **A failed build names its cause plainly.** With `npm install && echo QA84 && exit 3`:
   - the log ends with the command's output and `error: build command 'npm install && echo QA84 && exit 3' exited with code 3`;
   - `failureReason` contains `QA84` with no leading `2.413 `.
3. **Docker is unchanged.** A Docker-runtime service (`examples/hello-go`) still shows BuildKit's `#1 [internal] load build definition from Dockerfile` lines.
4. **Checks pass:** operator `make test`, and the backend suite if the narration moves there.

## Source + Goal linkage

- **Source:** the finding above (round 84 of the `/qa-find-bugs w4` loop, 2026-11-11). The durable evidence is the probe record; no screenshot was kept.
- **Goal linkage:**
  - [ADR010](../../../docs/ADR010-observability.md) § build logs and ADR004 (deploys).
  - `docs/render-artifacts/live-deploy-following.md` (Render's progress-line contract).
  - [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first).
- **Expected outcome:** on every native build, a user reads their own build command's output and a clear success or failure line, as on Render.
- **Why now:** every Node, Python, Ruby, Go, Rust and Elixir build on bex prints this, and it's the first thing a user reads when a build fails.
- **Render parity included (t005):** the log stream every surface reads (REST, GraphQL, MCP, the dashboard) changes.

## Post-release probe (QA loop round 143, 2026-11-11)

Pin `8330744bd643` (`9126d59d7`, 20:02 UTC) contains `a9c016955`. The probe ran on owned Free fixtures in `bex-canary`, deleted afterwards (DELETE 204, then GET 404):

- `qa-20261111-r143-node` (`srv-dbqelpofss2c73f2078g`, `render-examples/express-hello-world`, runtime `node`);
- `qa-20261111-r143-go` (`srv-dbqelq4o6rds73e87rd0`, `examples/hello-go`, runtime `docker`).

Logs were read with paged `GET /v1/logs?…&type=build&direction=forward`.

- **DoD 1 fails, on one clause.**
  - Passing: `dep-dbqelpofss2c73f20790` (build command `npm install`) went live. Its 40-line log reads `==> Using Node 24 (default)`, `==> Running build command 'npm install'...`, then npm's own lines unprefixed (`added 80 packages, and audited 81 packages in 2s`), then `==> Build successful 🎉`.
  - Failing: **20 lines contain `sha256:`**. They are the push phase's `Copying blob sha256:…` / `Copying config sha256:…` lines plus `Writing manifest to image destination`, all after the success line.
- **DoD 2 fails.**
  - Passing: for `dep-dbqenp8fss2c73f208gg` (`npm install && echo QA84 && exit 3`), the log shows `QA84` and `error: build command 'npm install && echo QA84 && exit 3' exited with code 3`.
  - Failing, the reason: **`failureReason` (REST and `deploy_ended`) is "build failed in the build environment step:" followed by `#15`–`#18` of bex's own Dockerfile** (`/opt/bex/pnpm.tgz`, `mwader/static-ffmpeg…@sha256:…`). It has no `QA84`.
  - Failing, the log: after the error line, the log prints `==> Building from …@039c347` and then the same transcript as `==> Build failed: …` / `==> #15 …` lines.
- **DoD 3 passes.** `dep-dbqelq4o6rds73e87rdg` (Docker) kept BuildKit's `#1 [internal] load build definition from Dockerfile` lines and went live.
- **Filed as t009.** It covers the failure-reason path (`failureDetail`, `lego/operator/internal/build/build.go:813-850`), the push digests and the narration order. t004 waits for it. w4/582 keeps the success line's position.

## Post-release probe — QA loop round 190 (2026-11-12, pin `a5be11e4e517`)

- t009's failure reproduces on a static site: `echo qa-break2; exit 3` gives `failureReason` "build failed in the build environment step:" followed by `#13`–`#17` of bex's toolchain (820 characters), with no command output and no exit code, while the log shows `error: build command 'echo qa-break2; exit 3' exited with code 3`.
- The verdict is missing because the kubelet keeps only the first 12 KiB ÷ (the pod's container count) bytes of each termination message (Kubernetes v1.33.0 `status_manager.go:1038-1055`). bex writes the transcript's last 4 KiB, so BuildKit's verdict at its end is cut. The full trace and the fix direction are in [t009's addendum](t009.md).
