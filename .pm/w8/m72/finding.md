# Native Python does not provide the documented Poetry toolchain

Why: a portable Python build that starts with poetry cannot run, even when the chosen Python and Poetry settings reach its environment correctly.

**Severity:** major runtime compatibility gap. **Attribution:** operator/native Python toolchain assembly. **Status:** todo. Functional `$qa-find-bugs-cli` w8/sweep307,2026-10-15 UTC. Sourcee7ae49c171fc1a05776bb6779ec42f582ada3c70; exact deployed backend/operator revision is unobserved. Published Bex0.3.2 and unchanged Render2.27/a764810a768202704e7206eb7b87a47211fcd98e, isolated human-QA bex-canary / tea-daif693dqjvc73e7as3g, non-TTY JSON. [Complete evidence](evidence/cli-sweep307/README.md).

## Reproduction and result

Use a fresh owned name/nonce and isolated QA config. The exact harmless authored settings follow; full original argv, raw valid native POSTs and complete redacted responses for both independent fixtures are retained in the evidence. The marker can be replaced consistently for a new trial.

```sh
AUTHORED_POETRY_BUILD='printf '"'"'%s\n'"'"' QA307_94da97_POETRY && poetry --version && python -c '"'"'import json,os,platform,subprocess
def probe_tool():
 try:
  r=subprocess.run(['"'"'"'"'"'"'"'"'poetry'"'"'"'"'"'"'"'"','"'"'"'"'"'"'"'"'--version'"'"'"'"'"'"'"'"'],text=True,capture_output=True,timeout=10)
  return {'"'"'"'"'"'"'"'"'error'"'"'"'"'"'"'"'"':None,'"'"'"'"'"'"'"'"'status'"'"'"'"'"'"'"'"':r.returncode,'"'"'"'"'"'"'"'"'stdout'"'"'"'"'"'"'"'"':r.stdout,'"'"'"'"'"'"'"'"'stderr'"'"'"'"'"'"'"'"':r.stderr}
 except FileNotFoundError:
  return {'"'"'"'"'"'"'"'"'error'"'"'"'"'"'"'"'"':'"'"'"'"'"'"'"'"'ENOENT'"'"'"'"'"'"'"'"','"'"'"'"'"'"'"'"'status'"'"'"'"'"'"'"'"':None,'"'"'"'"'"'"'"'"'stdout'"'"'"'"'"'"'"'"':'"'"'"'"'"'"'"'"''"'"'"'"'"'"'"'"','"'"'"'"'"'"'"'"'stderr'"'"'"'"'"'"'"'"':'"'"'"'"'"'"'"'"''"'"'"'"'"'"'"'"'}
with open('"'"'"'"'"'"'"'"'qa307-built.json'"'"'"'"'"'"'"'"','"'"'"'"'"'"'"'"'w'"'"'"'"'"'"'"'"') as f: json.dump({'"'"'"'"'"'"'"'"'marker'"'"'"'"'"'"'"'"':'"'"'"'"'"'"'"'"'QA307_94da97_POETRY'"'"'"'"'"'"'"'"','"'"'"'"'"'"'"'"'runtime'"'"'"'"'"'"'"'"':'"'"'"'"'"'"'"'"'poetry'"'"'"'"'"'"'"'"','"'"'"'"'"'"'"'"'version'"'"'"'"'"'"'"'"':platform.python_version(),'"'"'"'"'"'"'"'"'requested'"'"'"'"'"'"'"'"':os.environ.get('"'"'"'"'"'"'"'"'POETRY_VERSION'"'"'"'"'"'"'"'"'),'"'"'"'"'"'"'"'"'python_requested'"'"'"'"'"'"'"'"':os.environ.get('"'"'"'"'"'"'"'"'PYTHON_VERSION'"'"'"'"'"'"'"'"'),'"'"'"'"'"'"'"'"'probe'"'"'"'"'"'"'"'"':probe_tool()},f)
'"'"''
AUTHORED_PYTHON_START='python -c '"'"'import json,os,platform,subprocess
def probe_tool():
 try:
  r=subprocess.run(['"'"'"'"'"'"'"'"'poetry'"'"'"'"'"'"'"'"','"'"'"'"'"'"'"'"'--version'"'"'"'"'"'"'"'"'],text=True,capture_output=True,timeout=10)
  return {'"'"'"'"'"'"'"'"'error'"'"'"'"'"'"'"'"':None,'"'"'"'"'"'"'"'"'status'"'"'"'"'"'"'"'"':r.returncode,'"'"'"'"'"'"'"'"'stdout'"'"'"'"'"'"'"'"':r.stdout,'"'"'"'"'"'"'"'"'stderr'"'"'"'"'"'"'"'"':r.stderr}
 except FileNotFoundError:
  return {'"'"'"'"'"'"'"'"'error'"'"'"'"'"'"'"'"':'"'"'"'"'"'"'"'"'ENOENT'"'"'"'"'"'"'"'"','"'"'"'"'"'"'"'"'status'"'"'"'"'"'"'"'"':None,'"'"'"'"'"'"'"'"'stdout'"'"'"'"'"'"'"'"':'"'"'"'"'"'"'"'"''"'"'"'"'"'"'"'"','"'"'"'"'"'"'"'"'stderr'"'"'"'"'"'"'"'"':'"'"'"'"'"'"'"'"''"'"'"'"'"'"'"'"'}
from http.server import BaseHTTPRequestHandler,HTTPServer
with open('"'"'"'"'"'"'"'"'/opt/render/project/src/qa307-built.json'"'"'"'"'"'"'"'"') as f: built=json.load(f)
snapshot={'"'"'"'"'"'"'"'"'marker'"'"'"'"'"'"'"'"':'"'"'"'"'"'"'"'"'QA307_94da97_POETRY'"'"'"'"'"'"'"'"','"'"'"'"'"'"'"'"'runtime'"'"'"'"'"'"'"'"':'"'"'"'"'"'"'"'"'poetry'"'"'"'"'"'"'"'"','"'"'"'"'"'"'"'"'version'"'"'"'"'"'"'"'"':platform.python_version(),'"'"'"'"'"'"'"'"'requested'"'"'"'"'"'"'"'"':os.environ.get('"'"'"'"'"'"'"'"'POETRY_VERSION'"'"'"'"'"'"'"'"'),'"'"'"'"'"'"'"'"'python_requested'"'"'"'"'"'"'"'"':os.environ.get('"'"'"'"'"'"'"'"'PYTHON_VERSION'"'"'"'"'"'"'"'"'),'"'"'"'"'"'"'"'"'probe'"'"'"'"'"'"'"'"':probe_tool()}
snapshot['"'"'"'"'"'"'"'"'built'"'"'"'"'"'"'"'"']=built
class Handler(BaseHTTPRequestHandler):
 def do_GET(self):
  body=json.dumps(snapshot).encode()
  self.send_response(200)
  self.send_header('"'"'"'"'"'"'"'"'Content-Type'"'"'"'"'"'"'"'"','"'"'"'"'"'"'"'"'application/json'"'"'"'"'"'"'"'"')
  self.send_header('"'"'"'"'"'"'"'"'Content-Length'"'"'"'"'"'"'"'"',str(len(body)))
  self.end_headers()
  self.wfile.write(body)
HTTPServer(('"'"'"'"'"'"'"'"'0.0.0.0'"'"'"'"'"'"'"'"',int(os.environ.get('"'"'"'"'"'"'"'"'PORT'"'"'"'"'"'"'"'"','"'"'"'"'"'"'"'"'3000'"'"'"'"'"'"'"'"'))),Handler).serve_forever()
'"'"''
bex services create --name "$FRESH_OWNED_NAME" --type web_service --runtime python \
  --repo https://github.com/pallets/flask --branch main \
  --build-command "$AUTHORED_POETRY_BUILD" --start-command "$AUTHORED_PYTHON_START" \
  --env-var PYTHON_VERSION=3.12 --env-var POETRY_VERSION=2.1.3 \
  --plan free --region frankfurt --auto-deploy=false --confirm -o json

```

Both actual native commands exit0/HTTP201 and preserve runtime python, plan free, both version entries and exact command text. Both first deployments fail with `/bin/bash: line 2: poetry: command not found`, captured through native build history with BuildKit exit127. The standard-library fixture does not install project dependencies or require a Poetry-specific project manifest, so this fails at tool availability before any dependency/project operation.

Both same owned services then accept native control updates that use only Python's standard library. Both new deploys are Live/HTTP200. Sealed build snapshot and live process return Python3.12.15, requested PYTHON_VERSION=3.12 and POETRY_VERSION=2.1.3, while subprocess execution of poetry --version returns ENOENT in both. No source/repo/plan/version-setting change or manual installer workaround is used. Python selection and env delivery work; the documented tool is unavailable.

[Render's Poetry contract](https://render.com/docs/poetry-version) includes Poetry automatically in native Python and supports a compatible POETRY_VERSION override. [Poetry2.1.3's pinned primary metadata](https://github.com/python-poetry/poetry/blob/2.1.3/pyproject.toml) permits Python>=3.9,<4.0, including the tested3.12. The target is a supplied compatible Poetry at build/runtime and a reported resolved version, preserving the normal Python control. No independent authenticated Render run is claimed. An omitted-override/default hosted case remains unexercised; its default contract is source/documentation-established.

## Producer, consumer and correction

- Exact pinned cmd/servicecreate.go → pkg/service/create.go:45,95 carries both ordinary version entries; the raw forwarded POST proves this is valid unchanged upstream wire. Backend apps/rest.go:540,556–574 maps env and runtime into CreateRequest. Actual Python controls prove persistence and materialization; no new CLI flag/runtime enum or upstream fork is required.
- `lego/operator/internal/build/nativeversion.go:68–93`, especially Python's branch75–76, resolves PYTHON_VERSION/.python-version only. No POETRY_VERSION or default Poetry selection exists. `nativeresolve.go:48–69` decodes the full env bundle then rewrites only the primary FROM image. The actual paired source observations select Python3.12 correctly with/without Poetry but produce identical files and no tool setup.
- `native.go:97–103` inventories official Python primary images. `nativeDockerfile:234–273` copies source/loader into that primary image, directly executes the user's build and retains it for runtime CMD. It never provisions Poetry. This explains missing poetry in both build and the successful Python control's runtime; the shell correctly reports127.
- Extend native Python's resolve/assembly path to supply a reviewed Poetry toolchain by default and honor supported compatible POETRY_VERSION overrides. Keep primary Python selection independent and report unsupported combinations clearly through the existing named version-failure path. Materialize the selected tool before the authored build, retain its executable/dependencies at runtime, and narrate its chosen version/source.
- Keep the tool's own Python environment separate from application dependencies/virtualenvs. Preserve m61's exact command/env/PATH semantics, including explicit empty/narrow application PATH behavior and absolute invocation. Default setup must not overwrite a project-created environment. Tie selected tool identity to existing generated-build/cache inputs; verify changed/unchanged selections rather than asserting a new cache hash is required.
- Review the existing toolchain inventory/freshness path (`lego/operator/internal/build/toolchain-freshness.json`, `scripts/build-toolchain-freshness.sh`) for any added platform tool artifact. Coordinate shared native assembly/catalog edits with m71's Bun work while retaining this Python-specific default/compatibility contract. This schedules a concrete correction at those source functions; no proposed artifact or implementation was executed in QA.

## Shared callers, parity and checks

One production nativeDockerfile call is in nativeBuildPreparer at native.go:321; one production ResolveNativeBuild caller is cmd/native-resolve/main.go:45; nativeresolve.go:53 is the production ResolveNativeVersion caller. build.go:1125–1130 places preparer/resolver before BuildKit. Five reviewed Python primary lines exist (3.10–3.14);3.12 is live-tested. Audit every supported Python-native App consumer and the generic static/default-runtime boundary. Other primary runtimes, Dockerfile, prebuilt-image, buildpack and direct static publication must preserve their existing behavior; no live claim for those families follows from this Python trial.

Regression acceptance must execute an actual generated artifact: poetry --version works before the user build and in the resulting image; primary Python is still the selected line; supported override/default/unsupported combinations are coherent; application env/files/virtualenv/PATH behavior survives; selection changes and unchanged inputs behave correctly in cache. Version text appearing in a Dockerfile is insufficient. REST/GraphQL/MCP/Blueprint/dashboard ordinary env settings must reach that same behavior without a wire change. Full Poetry dependency resolution, project installation and other Python-line hosted cases remain verification tasks.

## Dedupe, estimate and cleanup

Exact whole-board Poetry/POETRY_VERSION searches across open/blocked/done items find no owner;74 actual open/blocked milestone READMEs were read with no match and targeted product history is empty. m51's original primary-Python version DoD stays satisfied by the3.12 control; m61 owns PATH/entrypoint,052 accurate command errors, and m71 optional Bun. Poetry's automatic Python default and its compatible override are independently reproduced here. Coordinate shared files with m71; no duplicate, CLI bug, regression of a previously tested Poetry path or inferred deploy-lag issue is filed.

Estimate190m/seven tasks: version/compatibility policy30, build/runtime provision45, shared/isolation/cache audit25, Render parity20, Simplify10, execution regressions40, closeout20. This exceeds the inbox sizing rule. No full operator/backend suite, raw production filesystem/CRI inventory, fixed hosted Poetry build, omitted-version hosted trial, other Python-line live trial or complete frontend/shared-write replay is claimed.

Two exact-owned services and four deploy children are fully cleaned by native delete0, service404, list absence and former runtime404; baseline6/quota6/terminating0 retained. After the implementation reaches production through the normal authorized workflow, repeat fresh two-client Poetry creates and actual build/runtime version markers, complete remaining shared/variant checks and clean exact-owned fixtures before closure. QA made no product edits, new ship, release, platform deployment or external submission. This filing is uncommitted.
