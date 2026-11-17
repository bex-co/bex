# w32 · m424 — Native Ruby and Python get the port defaults Render pins to 10000, and `GEM_PATH`, as Render documents

**Worker:** worker32 **Goal:** a native Ruby or Python service gets Render's `PASSENGER_PORT`, `GUNICORN_CMD_ARGS` (with `--bind`) and `GEM_PATH`, closing the three unsigned divergences ADR018 lists for them. **Status:** todo (t001–t006 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | `PASSENGER_PORT` follows the serving port on native Ruby services — **DONE** | 40m | — |
| [t002](done/t002.md) | `GUNICORN_CMD_ARGS` carries Render's `--bind`, at the serving port — **DONE** | 45m | — |
| [t003](done/t003.md) | `GEM_PATH` is Render's `/opt/render/project/.gems`, and Ruby's own gems stay visible — **DONE** | 40m | — |
| [t004](done/t004.md) | ADR018 and ADR060 replace the ⚠️ list with what shipped — **DONE** | 15m | t001, t002, t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 40m | t004 |
| [t007](t007.md) | Closeout (live replay on production) | 40m | t005, t006 |

## Definition of done

On production, in `bex-canary`, with owned free fixtures deleted afterwards:

1. A native Ruby web service whose start command is `bundle exec passenger start` (Passenger Standalone, builtin engine) and which configures no port reads `PASSENGER_PORT=10000` at runtime, listens on 10000, goes live on its first rollout and answers `200`. With its own `PASSENGER_PORT`, that value wins.
2. A native Python Flask service with a `gunicorn.conf.py` binding `0.0.0.0:8000` and start command `gunicorn app:app` reads `GUNICORN_CMD_ARGS=--preload --access-logfile - --bind=0.0.0.0:10000`, listens on 10000, goes live and answers `200`. Its own `GUNICORN_CMD_ARGS`, including an empty one, wins.
3. A native Ruby service reads `GEM_PATH=/opt/render/project/.gems` at build and runtime (or t003's recorded value), and `ruby -e 'require "minitest"'` succeeds.
4. ADR018's default-variables row lists none of the three as unsigned.
5. `make test` and `make lint` pass in `lego/operator/`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for Render parity of render.com/docs/environment-variables, 2026-11-16, item 2; claims re-verified the same day before filing.
- **Render reference:** render.com/docs/environment-variables (read 2026-11-16): Python `GUNICORN_CMD_ARGS=--preload --access-logfile - --bind=0.0.0.0:10000`; Ruby `GEM_PATH=/opt/render/project/.gems` and `PASSENGER_PORT=10000`.
- **bex today:** ADR018's default-variables row lists all three under "⚠️ Not set, not signed off"; their follow-ups (w8/113, w5/162) are closed, and w8/256 notes "Those reasons live in the code comment only" (`lego/operator/internal/build/native.go:468-475`).
- **Verified upstream behavior (2026-11-16):** Gunicorn applies `GUNICORN_CMD_ARGS` after the config file and before the command line (`gunicorn/app/base.py` `load_config`), and its `bind` default follows `PORT` (`gunicorn/config.py`). Passenger Standalone reads `PASSENGER_PORT`, default 3000, and does not document reading `PORT`. RubyGems keeps `GEM_HOME` (or the default dir) on `Gem.path`, and the docker-library Ruby bases set `GEM_HOME=/usr/local/bundle`, so the existing `GEM_PATH` concern is real on bex's images and t003 must keep Ruby's own gems visible.
- **Goal linkage:** ADR008 Render parity first; ADR060 native runtime parity.
- **Expected outcome:** a Passenger app from Render binds the routed port with no configuration; a Gunicorn config-file bind behaves as on Render; Ruby services see Render's `GEM_PATH`.
- **Why now:** AGENTS.md: a divergence stands only when ADR018 records it with the user's sign-off. These three have none, and nothing open tracks them.
- **Decision state:** following the serving port (equal to Render's 10000 whenever the port is the default, as `ROCKET_PORT` does since w8/273) is this proposal's recommendation, resting on ADR008; a fixed 10000 would move an app off a configured or detected port. It is not a recorded user decision. If t003 has to ship the trailing-separator `GEM_PATH`, that value difference waits on the user's sign-off in ADR018.
- **Render parity:** omitted: runtime image and operator environment only. These variables appear on no REST, GraphQL, MCP or dashboard surface, on Render or on bex.
