# w32 · m424 — Native Ruby and Python get the port defaults Render pins to 10000, and `GEM_PATH`, as Render documents

**Worker:** worker32 **Goal:** a native Ruby or Python service gets Render's `PASSENGER_PORT`, `GUNICORN_CMD_ARGS` (with `--bind`) and `GEM_PATH`, closing the three unsigned divergences ADR018 lists for them. **Status:** blocked — release gate, clears by itself: t001–t006 are done and shipped in `f1cdd052b`. [t007](t007.md) walks the definition of done on production once a "pin platform images to" commit contains it (§ Blocked).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | `PASSENGER_PORT` follows the serving port on native Ruby services — **DONE** | 40m | — |
| [t002](done/t002.md) | `GUNICORN_CMD_ARGS` carries Render's `--bind`, at the serving port — **DONE** | 45m | — |
| [t003](done/t003.md) | `GEM_PATH` is Render's `/opt/render/project/.gems`, and Ruby's own gems stay visible — **DONE** | 40m | — |
| [t004](done/t004.md) | ADR018 and ADR060 replace the ⚠️ list with what shipped — **DONE** | 15m | t001, t002, t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 40m | t004 |
| [t007](t007.md) | Closeout (live replay on production) — **BLOCKED** (release gate) | 40m | t005, t006 |

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

## Blocked (2026-11-17)

**Release gate (clears by itself; no decision needed).** t001–t006 shipped in `f1cdd052b` (operator: the native Python and Ruby images carry Render's `GUNICORN_CMD_ARGS=--preload --access-logfile - --bind=0.0.0.0:10000`, `PASSENGER_PORT=10000`, and `GEM_PATH=/opt/render/project/.gems` with `GEM_HOME` at the selected Ruby's `Gem.default_dir`; `servingPortDefault` puts the serving-port value on a web or private service's pod; ADR018 and ADR060). DoD items 4 and 5 already hold: ADR018's row lists only `NODE_MODULES_CACHE` as unset, and operator `make test` and `make lint` passed on 2026-11-17. Production runs the operator image of the newest `chore(deploy): pin platform images to <sha>` commit on `main`. That pin was `55c120858af0` when this milestone parked, and it does not contain `f1cdd052b`. Check:

```bash
git fetch origin main
pin=$(git log origin/main --grep='^chore(deploy): pin platform images to' -1 --format=%s | sed -E 's/.*images to ([0-9a-f]+).*/\1/')
git merge-base --is-ancestor f1cdd052b "$pin" && echo "pinned: walk the definition of done"
```

Then [t007](t007.md) walks DoD items 1–3 in `bex-canary` as the QA user (`bash scripts/qa-login.sh`), deleting every fixture afterwards with by-id `GET` 404 evidence. The fixtures need committed sources; none of `examples/` is a Passenger or a config-file Gunicorn app yet, so add them (a Ruby app with `Gemfile`, a committed `Gemfile.lock` with `passenger` and `rack`, and `config.ru`; a Flask app with `requirements.txt`, `app.py` and a `gunicorn.conf.py` binding `0.0.0.0:8000`) or use another public repository. The local gates `TestNativeRubyImagePassengerPort`, `TestNativePythonImageGunicornBindsRendersPort` and `TestNativeRubyImageGemPath` (`lego/operator/internal/build/nativeruntimeenv_test.go`) are the same shapes. Read the runtime values from the running instance (`bex ssh`, or `kubectl exec` on the control plane):

- Passenger, no port configured: build and runtime read `PASSENGER_PORT=10000`, the service goes live on its first rollout and answers `200`; a service `PASSENGER_PORT` wins.
- Flask with `gunicorn.conf.py` binding 8000 and `gunicorn app:app`: the pod reads `GUNICORN_CMD_ARGS=--preload --access-logfile - --bind=0.0.0.0:10000`, Gunicorn listens on 10000, the service goes live and answers `200`; a service value, and an empty one, win.
- A Ruby service: `GEM_PATH=/opt/render/project/.gems` at build and runtime, and `ruby -e 'require "minitest"'` succeeds in the instance. On production the image is built on the amd64 common base, where `RUBY_VERSION` is replayed from the selected Ruby image; the local gates ran on arm64 without it, so this is the first check of `GEM_HOME`'s expansion there (`ruby -e 'p Gem.dir == Gem.default_dir'` should print `true`).
