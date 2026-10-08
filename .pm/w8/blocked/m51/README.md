# w8 · m51 — Native builds honor the requested runtime version (Render's PYTHON_VERSION / NODE_VERSION / version-file contract)

**Worker:** worker8 **Goal:** a native-runtime build uses the toolchain version the service asks for, through Render's documented mechanisms, from a reviewed and digest-pinned set of version lines. It narrates which version it chose and why, and refuses an unsupported version by name. **Status:** blocked — t001–t008 done 2026-10-05; released and live-verified 2026-10-05 for env-var, engines, unsupported and default cases; t009 waits only on the `.python-version` / `.nvmrc` file-source replay (needs a fixture repo)

## Tasks (in order)

| id   | title                                                                           | est | depends_on                 |
| ---- | ------------------------------------------------------------------------------- | --- | -------------------------- |
| t001 | Decide the supported version lines and resolution policy (ADR060 D7-compatible) — **DONE** | 40m | —                          |
| t002 | Pinned per-line image matrix in the freshness inventory — **DONE**                         | 45m | w8/m51/t001                |
| t003 | Python and Node resolvers (env var → version files → engines → default) — **DONE**         | 1h  | w8/m51/t002                |
| t004 | Go, Ruby, Rust, Elixir resolvers — **DONE**                                                 | 1h  | w8/m51/t002                |
| t005 | Narrate the chosen version and refuse unsupported ones — **DONE**                          | 30m | w8/m51/t003, w8/m51/t004   |
| t006 | Render parity — **DONE**                                                                   | 30m | w8/m51/t005                |
| t007 | Simplify — **DONE**                                                                        | 20m | w8/m51/t006                |
| t008 | Test coverage — **DONE**                                                                   | 45m | w8/m51/t006                |
| t009 | Closeout                                                                        | 15m | w8/m51/t007, w8/m51/t008   |

## Definition of done

On production, through the released `bex` CLI:

- A native Python web service with `PYTHON_VERSION=3.11.9` builds on a Python 3.11 toolchain. Its build log shows `python --version` → `Python 3.11.x` and a narration line naming the version and its source (`PYTHON_VERSION`). Repeated with a `.python-version` file instead of the env var, the build uses the file.
- A native Node service with `NODE_VERSION=22` (and separately a `.nvmrc`, and an `engines.node` range with an upper bound) builds on Node 22.
- An unsupported request (e.g. `PYTHON_VERSION=2.7.18`) fails the build with a named, tenant-classified message listing the supported lines. It never silently falls back to the default.
- A service with no version signal keeps today's default line. Its narration says "default".
- Every image used is digest-pinned and listed in `toolchain-freshness.json`, and `scripts/build-toolchain-freshness.sh validate` passes.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` w8 loop, 2026-10-04 sweep 12. Live, production pin `8f58b933d`, `bex v0.2.1`, workspace `bex-canary`, human device login. Free native web `qa-20261004-…-pyver` (`srv-db1ivq0cnepc739umea0`, deleted), `--runtime python`, `--env-var PYTHON_VERSION=3.11.9`, build command `echo "QAVER python=$(python --version 2>&1) want=$PYTHON_VERSION"; pip install -r requirements.txt` → build log `QAVER python=Python 3.13.15 want=3.11.9`. The deploy went live on the wrong toolchain with no warning.
- **Render contract** (fetched 2026-10-04): render.com/docs/python-version uses precedence `PYTHON_VERSION` (fully qualified) → `.python-version` (minor allowed) → creation-date default (currently 3.14.3), minimum 3.7.3. render.com/docs/node-version uses `NODE_VERSION` → `.node-version` → `.nvmrc` → `package.json` `engines`, semver ranges and aliases (`lts`) via `node-version-alias`, and a creation-date default (currently 24.21.0). Confirm the Go/Ruby/Rust/Elixir rules from their Render docs pages in t001 before implementing t004.
- **Mechanism:** `lego/operator/internal/build/native.go:78-85` `nativeRuntimeImages` maps each runtime to exactly one digest-pinned image (`python:3.13-bookworm@sha256:…`, `node:24-bookworm@…`, `golang:1.24-bookworm@…`, …), and `nativeDockerfile` (`:193`) always uses it. No code reads a version signal. The buildpack path (`builder: buildpack`, w1/m169, w1/121) already honors `engines.node`/`BP_*`, so the gap is specific to `runtime: <native>`, which is what the pinned CLI's `--runtime` and Render-shaped Blueprints select.
- **Constraint:** ADR060 D7 makes native bases a reviewed, digest-pinned inventory (`toolchain-freshness.json`, weekly freshness workflow), because a floating tag in a privileged tenant build environment is a reliability and security defect. Version selection must stay inside that model: a finite set of reviewed lines, never an arbitrary tag pulled at build time.
- **Goal linkage:** ADR018/ADR049 Render parity for native services. A Render user's existing `PYTHON_VERSION`/`.nvmrc` must mean the same thing on bex (pillar: drop-in Render alternative, ADR008).
- **Expected outcome:** apps pinned to a non-default runtime version (very common: Django on 3.11, older Node LTS) build on the toolchain they declare, or fail loudly, instead of building on the wrong one and breaking at runtime.
- **Why now:** this is silent. A wrong-major build can pass and then crash or misbehave in production with nothing pointing at the cause. The fix also gets harder the longer tenants depend on the accidental single version.
- **Render parity included:** the change is tenant-visible (build behavior, build-log narration, possibly a dashboard runtime-version hint), so t006 checks REST/GraphQL/MCP/dashboard and the Blueprint path.

## Live verification (2026-10-05, `/qa-find-bugs-cli` w8 loop, sweep 23)

Production pin `c7afefad5` (contains `dfb81512d`), `bex v0.2.1`, workspace `bex-canary`, human device login. Fixtures (both deleted): native Python `srv-db1nab8ti8qc73bltbcg` (`examples/hello-python`) and native Node `srv-db1nabm7q7bs739ppan0` (`examples/hello-node`). The build command echoes the toolchain version.

| DoD item                         | Result |
| -------------------------------- | ------ |
| `PYTHON_VERSION=3.11.9`          | ✅ `==> Using Python 3.11 (from PYTHON_VERSION=3.11.9)`; `python --version` → `Python 3.11.17`; serves 200 |
| `NODE_VERSION=22`                | ✅ `==> Using Node 22 (from NODE_VERSION=22)`; `node --version` → `v22.23.3`; serves 200 |
| `engines.node` bounded/unbounded | ✅ (unbounded) `examples/hello-node` `engines.node: ">=20"`, no `NODE_VERSION` → `==> Using Node 26 (from package.json engines.node)`, `v26.10.0`, Render's "unbounded resolves to latest". A bounded range was not exercised. |
| Unsupported request              | ✅ `PYTHON_VERSION=2.7.18` → `build_failed`: `==> Build failed: build failed in the runtime version selection step: PYTHON_VERSION=2.7.18 is not a supported Python version; supported Python lines are 3.10, 3.11, 3.12, 3.13, 3.14`. The prior release kept serving. |
| No version signal                | ✅ `==> Using Python 3.13 (default)`; `Python 3.13.15` |
| `.python-version` file           | ⏳ not exercised: needs a repo containing the file (none of `bex-co/bex`'s examples has one) |
| `.nvmrc` / `.node-version` file  | ⏳ not exercised, same reason |
| Freshness `validate`             | not re-run here (CI) |

Remaining before t009: one live build from a repo carrying `.python-version` (e.g. `3.12`) and one carrying `.nvmrc`, ideally on a QA-owned fork or fixture repo.

## Fixtures added (2026-10-08, w8 /loopx)

The "needs a repo carrying the file" half of the gate is cleared. `bex-co/bex` is public and now has [`examples/runtime-version/`](../../../../examples/runtime-version/README.md): `python/` with `.python-version` = `3.12` (default 3.13), and `node/` with `.nvmrc` = `22` (default 24). Each has no env var and no `engines`, so only the file can select the line. The README gives the exact `bex services create` commands and the expected narration. **Remaining gate (user):** an authenticated production run of both builds, then t009. The QA device login demands a password reauth the agent must not perform (see m45).
