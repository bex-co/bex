# Render's native runtime version lines in the build log

Collected on 2026-11-21 for w9/139 from build logs that Render users published. No Render account was used. Each quote below is copied from the cited page, and the date is the post's date. Community forum threads (`community.render.com`, which now redirects to `render.discourse.group`) were read from their Wayback Machine copies, because that host did not resolve from this network. The GoRails thread was also read from its Wayback copy (2025-12-04), because the live page answered 500.

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

A Bun lockfile opts a service into Bun without naming a version. Render then installs its default ([bun-version](https://render.com/docs/bun-version): "If you don't set a Bun version for your service, Render's default version depends on when you originally created the service"), and its line reads `(default)`.

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

## Not measured

No published Render log shows these signals, so bex keeps its own line, for example `==> Using Bun 1.3.14 (from BUN_VERSION=1.3)`:

- Ruby: `gems.locked` and `.tool-versions`.
- Elixir: `ELIXIR_VERSION` and `ERLANG_VERSION`, alone or together.
- Bun: `BUN_VERSION` and `.bun-version`.
- Poetry: `POETRY_VERSION`.
- uv: `UV_VERSION`.
- Rust: every signal (`RUSTUP_TOOLCHAIN`, `rust-toolchain`, `rust-toolchain.toml` and the default).
- Go: only the 2023 line above is on record. It embeds `go version`'s output, including the release and the architecture, which only the build step knows. bex keeps `==> Using Go 1.27 (default)`.
- Where a Go service's Node companion version file is named: Render keeps a Go service's source in its `GOPATH` (render.com/docs/disks), and bex names the file there.

## Seen but not adopted

These lines appear in the same logs. bex does not print them yet:

- `==> Docs on specifying a Node.js version: https://render.com/docs/node-version` follows each version line, with the runtime's own page: `a Python version`, `a Ruby version`, `a bun version`, `a Poetry version`, `a uv version`, and `Elixir and Erlang versions` once for the pair.
- `==> Requesting Node.js version >=20.15` comes before the `Using` line for a `package.json` `engines.node` range ([n8n-io/n8n#11313](https://github.com/n8n-io/n8n/issues/11313), 2024-10-18; [vitejs/vite#18663](https://github.com/vitejs/vite/discussions/18663), 2024-11-13). No log shows whether an exact `engines` value prints it.
- `==> Installing Python version 3.13.7...`, `==> Installing Poetry version 2.1.3` and `==> Installing uv version 0.7.12...` appear when Render installs a release it does not have cached.
- A 2024 Python service logged `==> Using Bun version 1.1.0 (default)` with no Bun signal. Render's current page says a service with no Bun version and no Bun lockfile has no Bun.
