# Render's native runtime version lines in the build log

Collected on 2026-11-21 for w9/139 from build logs that Render users published. No Render account was used. Each quote below is copied from the cited page, and the date is the post's date. Community forum threads (`community.render.com`, which now redirects to `render.discourse.group`) were read from their Wayback Machine copies, because that host did not resolve from this network. The GoRails thread was also read from its Wayback copy (2025-12-04), because the live page answered 500. w9/141 added the lines Render prints around the version line, the same day and from the same kinds of sources. Where a log does not state the value behind a signal, it was read from the repository at the commit the log checks out.

## The shape of Render's line

Every quoted line has this form:

```
==> Using <Name> version <version> <how>
```

- `<how>` is `(default)` when nothing names a version.
- It is `via environment variable <NAME>` for an environment variable. The variable's value is not printed.
- It is `via <path>` for a version file. The path is the file's absolute path in the build, `/opt/render/project/src/<file>`. For a service with a root directory, the path includes that directory: a Python service with root directory `backend` logged `Using Python version 3.12.14 via …/backend/.python-version` (the author shortened the prefix).
- `<Name>` is `Node.js`, `Python`, `Ruby`, `Bun`, `Poetry`, `uv`, `Elixir` or `Erlang`. Node's line read `Node` until about October 2024 (`==> Using Node version 20.15.1 (default)`, 2024-09-01). Every later log reads `Node.js` (`==> Using Node.js version 22.10.0 via environment variable NODE_VERSION`, 2024-10-21).

## Measured lines

| Runtime | Signal | Render's line | Source |
| --- | --- | --- | --- |
| Node | `NODE_VERSION` | `==> Using Node.js version 22.10.0 via environment variable NODE_VERSION` | [n8n-io/n8n#11313](https://github.com/n8n-io/n8n/issues/11313#issuecomment-2427524896), 2024-10-21 |
| Node | `.node-version` | `Using Node.js version 24.22.1 via /opt/render/project/src/.node-version` | [rizwandar/Performance-Management#408](https://github.com/rizwandar/Performance-Management/pull/408), 2026-10-27; [PIN-devel/innova-platform#39](https://github.com/PIN-devel/innova-platform/pull/39), 2026-10-27 |
| Node | `.nvmrc` | `==> Using Node.js version 24.21.0 via /opt/render/project/src/.nvmrc` | [fabogit/battleship#49](https://github.com/fabogit/battleship/pull/49), 2026-10-05; [vicgopfert/penumbra#18](https://github.com/vicgopfert/penumbra/pull/18), 2026-10-22; [syed-waleed-ahmed/Portfolio#64](https://github.com/syed-waleed-ahmed/Portfolio/pull/64), 2026-11-20 |
| Node | `package.json` `engines.node` | `==> Using Node.js version 26.8.2 via /opt/render/project/src/package.json` | [Kanishkrawatt/Sort-Image-by-Face#2](https://github.com/Kanishkrawatt/Sort-Image-by-Face/pull/2), 2026-09-14; [n8n-io/n8n#11313](https://github.com/n8n-io/n8n/issues/11313#issuecomment-2424222127), 2024-10-19 |
| Node | none | `==> Using Node version 20.15.1 (default)`, from before the rename | [rdavydov/Twitch-Channel-Points-Miner-v2#592](https://github.com/rdavydov/Twitch-Channel-Points-Miner-v2/issues/592), 2024-09-01; a 2026 PR describes its service's Node as "Node.js 24.21.0(default)" ([PIN-devel/innova-platform#39](https://github.com/PIN-devel/innova-platform/pull/39)) |
| Python | `PYTHON_VERSION` | `==> Using Python version 3.11.6 via environment variable PYTHON_VERSION` | [community.render.com/t/17572](https://community.render.com/t/python-3-11-6-is-failing-on-deployment/17572), December 2023; [community.render.com/t/23709](https://community.render.com/t/first-dash-app-build-fails-due-to-setuptools/23709), 2024 |
| Python | `.python-version` | `==> Using Python version 3.13.7 via /opt/render/project/src/.python-version` | [community.render.com/t/39071](https://community.render.com/t/installing-uv-version-0-7-12-command-not-found-error/39071), log of 2025-09-04; root directory: [mitangy/optcg-deck-planner#132](https://github.com/mitangy/optcg-deck-planner/pull/132), 2026-09-28 |
| Python | none | `Using Python version 3.14.3 (default)` | [mitangy/optcg-deck-planner#132](https://github.com/mitangy/optcg-deck-planner/pull/132), 2026-09-28; `==> Using Python version 3.11.7 (default)`, [chartbrew/chartbrew#202](https://github.com/chartbrew/chartbrew/issues/202), 2024-02-22 |
| Ruby | `Gemfile.lock` | `==> Using Ruby version 3.4.1 via /opt/render/project/src/Gemfile.lock` | [GoRails forum](https://gorails.com/forum/running-into-render-build-error-related-to-tailwind), 2025-01-05 |
| Ruby | `.ruby-version` | `==> Using Ruby version 3.4.2 via /opt/render/project/src/.ruby-version` | [seunghan.xyz](https://seunghan.xyz/posts/render-rails-monorepo-ruby-version-solidqueue-puma/), 2026-04-29; `3.2.2`, [community.render.com/t/17928](https://community.render.com/t/web-service-issue-with-rails-api/17928), 2023-12-22 |
| Ruby | `Gemfile` `ruby` | `Using Ruby version ".tool-versions" via /opt/render/project/src/Gemfile` (a `ruby file:` directive read as a version, the bug Render fixed on 2024-04-04) | [feedback.render.com](https://feedback.render.com/features/p/use-gemfilelock-or-tool-versions-instead-of-gemfile-for-ruby-version-detection), 2024-04-01 |
| Ruby | none | `==> Using Ruby version 3.4.4 (default)` | [seunghan.xyz](https://seunghan.xyz/posts/render-rails-monorepo-ruby-version-solidqueue-puma/), 2026-04-29; `3.2.2 (default)`, [chartbrew/chartbrew#202](https://github.com/chartbrew/chartbrew/issues/202), 2024-02-22 |
| Elixir | none | `==> Using Erlang version 26.2.2 (default)`, then `==> Using Elixir version 1.16.1 (default)` | [elixirforum.com/t/65411](https://elixirforum.com/t/issues-deploying-elixir-phoenix-app-on-render-com-due-to-missing-dependencies/65411), 2024-08-09 |
| Bun | none | `==> Using Bun version 1.1.0 (default)` | [rdavydov/Twitch-Channel-Points-Miner-v2#592](https://github.com/rdavydov/Twitch-Channel-Points-Miner-v2/issues/592), 2024-09-01 |
| Poetry | none | `==> Using Poetry version 2.1.3 (default)` | [community.render.com/t/39071](https://community.render.com/t/installing-uv-version-0-7-12-command-not-found-error/39071), 2025-09-04; `1.7.1 (default)`, [chartbrew/chartbrew#202](https://github.com/chartbrew/chartbrew/issues/202) |
| uv | `uv.lock`, no `UV_VERSION` | `==> Using uv version 0.7.12 (default)` | [community.render.com/t/39071](https://community.render.com/t/installing-uv-version-0-7-12-command-not-found-error/39071), 2025-09-04. Render adds uv only for a `uv.lock` ([uv-version](https://render.com/docs/uv-version)) |
| Go | none (Render offers no selection) | `==> Using Go version go version go1.20.5 linux/amd64` | [linweiyuan/go-chatgpt-api#266](https://github.com/linweiyuan/go-chatgpt-api/issues/266), 2023-09-28 |

A Bun lockfile opts a service into Bun without naming a version. Render then installs its default ([bun-version](https://render.com/docs/bun-version): "If you don't set a Bun version for your service, Render's default version depends on when you originally created the service"), and its line reads `(default)`. On a native Node service, Render also writes that Bun version line and its docs line before `==> Running '<command>'` at each start of the instance (Pranav7758/AniCircle at `5fac202ebc`, harshagavadi/akshay at `dbfd3ed4b1`, Bubba898/luchadores-web#5); since w32/093, bex's native Node build writes the resolved Bun pair (for `bun.lock`, `bun.lockb`, `.bun-version` or `BUN_VERSION`, without build-time notes) into `/opt/bex/native-init-lines`, and `native-init` prints it before `==> Running '<command>'` on every start. Other runtimes and Node services without Bun write none at start.

## Lines around the version line

### The docs line

Each runtime's version line is followed by a line that points to the page on choosing that runtime's version:

```
==> Docs on specifying a <runtime> version: https://render.com/docs/<page>
```

The logs show it after a default, an environment variable and a version file alike. Elixir and Erlang share one line, printed after the Elixir line.

| Runtime | Render's line | Source |
| --- | --- | --- |
| Node | `==> Docs on specifying a Node.js version: https://render.com/docs/node-version` | after `via …/package.json`, [n8n-io/n8n#11313](https://github.com/n8n-io/n8n/issues/11313#issuecomment-2424222127), 2024-10-19; after `via environment variable NODE_VERSION`, [thedimlebowski/soakup#35](https://github.com/thedimlebowski/soakup/issues/35), 2026-06-12; after `(default)`, [thephm/the-beans#417](https://github.com/thephm/the-beans/issues/417), 2026-09-30. Until about October 2024 it read `a Node version` ([beeminder/blog#292](https://github.com/beeminder/blog/issues/292), 2023-08-11) |
| Python | `==> Docs on specifying a Python version: https://render.com/docs/python-version` | after `PYTHON_VERSION`, [community.render.com/t/17572](https://community.render.com/t/python-3-11-6-is-failing-on-deployment/17572), December 2023; after `.python-version`, [community.render.com/t/39071](https://community.render.com/t/installing-uv-version-0-7-12-command-not-found-error/39071), 2025-09-04; after `(default)`, [chartbrew/chartbrew#202](https://github.com/chartbrew/chartbrew/issues/202), 2024-02-22 |
| Ruby | `==> Docs on specifying a Ruby version: https://render.com/docs/ruby-version` | after `.ruby-version`, [community.render.com/t/17928](https://community.render.com/t/web-service-issue-with-rails-api/17928), 2023-12-22; after `(default)`, [chartbrew/chartbrew#202](https://github.com/chartbrew/chartbrew/issues/202), 2024-02-22 |
| Elixir and Erlang | `==> Docs on specifying Elixir and Erlang versions: https://render.com/docs/elixir-erlang-versions`, once, after the Elixir line | [elixirforum.com/t/65411](https://elixirforum.com/t/issues-deploying-elixir-phoenix-app-on-render-com-due-to-missing-dependencies/65411), 2024-08-09 |
| Bun | `==> Docs on specifying a Bun version: https://render.com/docs/bun-version` | [halc8312/shinwa#105](https://github.com/halc8312/shinwa/issues/105), 2025-07-12; [ThePravinDeshmukh/rag#1](https://github.com/ThePravinDeshmukh/rag/issues/1), 2025-08-07; [Bubba898/luchadores-web#5](https://github.com/Bubba898/luchadores-web/issues/5), 2026-02-01. A 2024 log reads `a bun version` ([rdavydov/Twitch-Channel-Points-Miner-v2#592](https://github.com/rdavydov/Twitch-Channel-Points-Miner-v2/issues/592), 2024-09-01) |
| Poetry | `==> Docs on specifying a Poetry version: https://render.com/docs/poetry-version` | [chartbrew/chartbrew#202](https://github.com/chartbrew/chartbrew/issues/202), 2024-02-22; [community.render.com/t/39071](https://community.render.com/t/installing-uv-version-0-7-12-command-not-found-error/39071), 2025-09-04; [frayel/devagent#62](https://github.com/frayel/devagent/issues/62), 2026-10-01 |
| uv | `==> Docs on specifying a uv version: https://render.com/docs/uv-version` | [community.render.com/t/39071](https://community.render.com/t/installing-uv-version-0-7-12-command-not-found-error/39071), 2025-09-04 |
| Go | none: `==> Running build command …` follows the Go line | [linweiyuan/go-chatgpt-api#266](https://github.com/linweiyuan/go-chatgpt-api/issues/266), 2023-09-28 |

### The Node request

When the Node version a service asks for is a range or an alias, Render echoes the request, as the service wrote it, on the line before the `Using` line:

```
==> Requesting Node.js version >=22.12.0
==> Using Node.js version 26.3.0 via /opt/render/project/src/package.json
==> Docs on specifying a Node.js version: https://render.com/docs/node-version
```

That is [render-examples/workflow-agents-workshop-ts#1](https://github.com/render-examples/workflow-agents-workshop-ts/issues/1) (2026-06-17), whose `package.json` has `"node": ">=22.12.0"`.

| Request | Render's first line | Source |
| --- | --- | --- |
| `engines.node` `>=20.15` | `==> Requesting Node.js version >=20.15` | [n8n-io/n8n#11313](https://github.com/n8n-io/n8n/issues/11313#issuecomment-2424222127), 2024-10-19 |
| `engines.node` `>= 20.4.0` | `==> Requesting Node.js version >= 20.4.0`, with the space as written | [louislam/uptime-kuma#6275](https://github.com/louislam/uptime-kuma/issues/6275), 2025-10-28 |
| `engines.node` `>=22.0.0` | `==> Requesting Node.js version >=22.0.0` | [OscarCasadoLorenzo/SeaPunk-Manager#31](https://github.com/OscarCasadoLorenzo/SeaPunk-Manager/issues/31), 2025-11-14 |
| `NODE_VERSION=20.x` | `==> Requesting Node.js version 20.x`, then `==> Using Node.js version 20.19.5 via environment variable NODE_VERSION` | [muppetbrown/mana_meeples_singles_market#226](https://github.com/muppetbrown/mana_meeples_singles_market/issues/226), 2025-10-19 |
| `.nvmrc` `lts/*` | `==> Requesting node version lts/*`, then `==> Using Node version 22.11.0 via /opt/render/project/src/.nvmrc`. Both lines are in the wording from before the `Node.js` rename | [FerryTempo/FTServer#103](https://github.com/FerryTempo/FTServer/issues/103), 2024-11-08 |
| `NODE_VERSION=22.22.3`, exact | none: the `Using` line comes first | [thedimlebowski/soakup#35](https://github.com/thedimlebowski/soakup/issues/35), 2026-06-12; the value is in its `render.yaml` |
| `.nvmrc` `18.16.0`, exact | none | [beeminder/blog#292](https://github.com/beeminder/blog/issues/292), 2023-08-11 |

## bex's lines

bex's resolve phase (`NativeChoice.NarrationIn`, `lego/operator/internal/build/nativeversion.go`) prints Render's form for each signal in the table above. Version files are named under the service's source directory in the image, which the operator passes to the resolve phase as `BEX_NATIVE_PROJECT_DIR`. The Erlang line for an Elixir default comes before the Elixir line, as on Render, when bex installs Render's exact default OTP. Otherwise bex does not know the image's OTP release and prints only the Elixir line.

| Signal | bex before w9/139 | bex now |
| --- | --- | --- |
| `NODE_VERSION=22` | `==> Using Node 22.23.5 (from NODE_VERSION=22)` | `==> Using Node.js version 22.23.5 via environment variable NODE_VERSION` |
| `.nvmrc` | `==> Using Node 18.20.8 (from .nvmrc)` | `==> Using Node.js version 18.20.8 via /opt/render/project/src/.nvmrc` |
| `Gemfile.lock` | `==> Using Ruby 3.3.14 (from Gemfile.lock)` | `==> Using Ruby version 3.3.14 via /opt/render/project/src/Gemfile.lock` |
| none (Elixir, Render's OTP) | `==> Using Elixir 1.18.4 on Erlang/OTP 28.0.2 (default)` | `==> Using Erlang version 28.0.2 (default)` and `==> Using Elixir version 1.18.4 (default)` |
| `bun.lock` | `==> Using Bun 1.4.5 (from bun.lock)` | `==> Using Bun version 1.4.5 (default)` |
| `uv.lock` | `==> Using uv 0.13.7 (from uv.lock)` | `==> Using uv version 0.13.7 (default)` |

Since w9/141 the resolve phase also prints Render's lines around the version line:

- **The docs line.** Each Node, Python, Ruby, Elixir, Bun, Poetry and uv version line is followed by that runtime's line from the table above. It also follows bex's own line for a signal no Render log shows, because Render's logs show the docs line after every kind of signal. A Go or Rust line has none.
- **The Node request.** A Node range or alias from any Node signal is echoed first, as the service wrote it. Surrounding whitespace is trimmed, and the request is cut at 120 characters. An exact version is not echoed.
- **bex's own notes,** such as `==> No official image node:22.1.5-bookworm; using the reviewed Node 22 image`, come after the docs line. That is where Render's `Installing …` line appears.

```
==> Requesting Node.js version ^20
==> Using Node.js version 20.20.2 via /opt/render/project/src/package.json
==> Docs on specifying a Node.js version: https://render.com/docs/node-version
```

## Not measured

No published Render log shows these signals, so bex keeps its own line, for example `==> Using Bun 1.3.14 (from BUN_VERSION=1.3)`:

- Ruby: `gems.locked` and `.tool-versions`.
- Elixir: `ELIXIR_VERSION` and `ERLANG_VERSION`, alone or together.
- Bun: `BUN_VERSION` and `.bun-version`.
- Poetry: `POETRY_VERSION`.
- uv: `UV_VERSION`.
- Rust: every signal (`RUSTUP_TOOLCHAIN`, `rust-toolchain`, `rust-toolchain.toml` and the default).
- Go: only the 2023 line above is on record. A search of GitHub issues on 2026-11-21 found no later Render log with a Go line. It embeds `go version`'s output, including the release and the architecture, which only the build step knows. bex keeps `==> Using Go 1.27 (default)`, with no docs line, as in the 2023 log.
- The Node request for a partial version, such as `NODE_VERSION=22` or `.nvmrc` `18.17`: no published log shows one, so bex echoes none. Render resolves ranges and aliases with the `node-version-alias` module ([node-version](https://render.com/docs/node-version)), which reads a partial version as a range too.
- The current wording of the Node request for an alias. The only log of one (`lts/*`, 2024-11-08) uses the wording from before the rename, on both its lines. bex prints the current wording, `==> Requesting Node.js version lts/*`, which a range gets.
- Where a Go service's Node companion version file is named: Render keeps a Go service's source in its `GOPATH` (render.com/docs/disks), and bex names the file there.

## Seen but not adopted

These lines appear in the same logs. bex does not print them:

- `==> Installing Node.js version 26.3.0...` (`renderlog.BuildRuntime` blue in `ivanseibel/tretta` `04b-render-logs.json` at `b0f216c985` for `22.23.1...`; also `thedimlebowski/soakup#35` for `22.22.3...`, `render-examples/workflow-agents-workshop-ts#1` for `26.3.0...`), `==> Installing Python version 3.11.15...` (`abhi208361-alt/AI-Based-SOC-with-Automated-Firewall-Response-2.0#11`; `3.13.7...` in `community.render.com/t/39071`; `3.11.5...` before `Using` in `alexsandovalball/tombrady420#109`), `==> Installing Poetry version 1.8.3` (no ellipsis; `peacefulseeker/django-libraryms#4`, `chartbrew/chartbrew#202` for `1.7.1`), and `==> Installing uv version 0.10.2...` (`Higashi-Masafumi/jaXiv#109`, `0.7.12...` in `community.render.com/t/39071`). Render prints these when its build container does not already have the requested version pre-baked — even on cached deploys (`tretta` and `soakup#35` both restored cache first) — and omits them for pre-baked defaults (`manishgupta2026/TG-Drive#8`, `grimtear/Test-Sheet-Dashboard#1`, default Poetry `2.1.3` in `tombrady420#109`). On bex (w32/098), `nativeCommonBase` carries no pre-installed primary runtime: `resolve-native-runtime` runs before BuildKit and rewrites `/native/Dockerfile` to copy every primary runtime and Bun from a digest-pinned prebuilt OCI image (`COPY --link --from=…`), so no in-build runtime installer ever runs for Node.js, Python, Ruby, Elixir, Go, Rust or Bun, and BuildKit either pulls or imports those OCI layers from `--import-cache`. Poetry and uv are installed by BuildKit `RUN … pip install` layers in the subsequent `buildkit` container after `resolve-native-runtime` has already emitted every runtime and companion's `Using`/`Docs` lines (including Python's Node.js companion), and BuildKit emits `#N <step>` before `#N CACHED`, so emitting `Installing` there would misorder the log or misreport cached layers. See [ADR018](../ADR018-render-parity.md) § Logs, Build logs.
- A 2024 Python service logged `==> Using Bun version 1.1.0 (default)` with no Bun signal. Render's current page says a service with no Bun version and no Bun lockfile has no Bun.
