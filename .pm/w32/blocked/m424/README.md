# w32 · m424 — Native Ruby and Python get the port defaults Render pins to 10000, and `GEM_PATH`, as Render documents

**Worker:** worker32 **Goal:** a native Ruby or Python service gets Render's `PASSENGER_PORT`, `GUNICORN_CMD_ARGS` (with `--bind`) and `GEM_PATH`, closing the three unsigned divergences ADR018 lists for them. **Status:** blocked — DoD 1 fails on production for a reason outside this milestone: Passenger Standalone cannot start on any native Ruby service, because its watchdog needs `CAP_SETGID`, which native runtimes do not get ([w32/m462](../../m462/README.md) (promoted from w32/038)). t001–t006 are done, and DoD 2–5 hold. [t007](t007.md) reruns the Passenger walk once w32/038 is on production (§ Blocked on w32/038).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | `PASSENGER_PORT` follows the serving port on native Ruby services — **DONE** | 40m | — |
| [t002](done/t002.md) | `GUNICORN_CMD_ARGS` carries Render's `--bind`, at the serving port — **DONE** | 45m | — |
| [t003](done/t003.md) | `GEM_PATH` is Render's `/opt/render/project/.gems`, and Ruby's own gems stay visible — **DONE** | 40m | — |
| [t004](done/t004.md) | ADR018 and ADR060 replace the ⚠️ list with what shipped — **DONE** | 15m | t001, t002, t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 40m | t004 |
| [t007](t007.md) | Closeout (live replay on production) — **BLOCKED** (w32/038) | 40m | t005, t006 |

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

## Blocked on the release gate (2026-11-17, cleared)

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

## Unblocked (2026-11-17)

**Source:** verified evidence (`/pm unblock`, `/loopx w32` worker). **Affected:** t007. **Finding:** the release gate cleared. The newest pin commit on `main` is `336b647b1 chore(deploy): pin platform images to 6984548a30f8` (10:34 UTC), and `git merge-base --is-ancestor f1cdd052b 6984548a30f8` succeeds. That commit pins `ghcr.io/bex-co/bex-operator@sha256:4b03796361d1…`. At 10:38 UTC production still ran the previous digest (`dd7d254370a0…`) until Argo CD's `bex-platform-prod` sync updated the `bex-operator` Application; from 10:38:43 UTC `bex-controller-manager` (whose own image is the build Job's native resolver), `bex-api`, `bex-activator`, `bex-ssh-gateway` and the other `bex-system` Deployments run `sha256:4b03796361d1…`, checked with `kubectl` on the control plane. **Remaining gates:** none. **Next:** t007 walks DoD items 1–3 in `bex-canary` as the QA user with Passenger and Gunicorn fixtures, records Python's `GUNICORN_CMD_ARGS` and Ruby's `PASSENGER_PORT` and `GEM_PATH` at build and runtime for w32/m425's t007 to reuse, deletes every fixture with by-id `GET` 404 evidence, and closes the milestone.

## Walk (t007, 2026-11-17)

Production pin `6984548a30f8` (`336b647b1`). From 10:38:43 UTC, `bex-controller-manager` and `bex-api` ran `ghcr.io/bex-co/bex-operator@sha256:4b03796361d1…`, and the fixtures' build Jobs ran their `resolve-native-runtime` step on the same digest. The walk ran 10:42–11:00 UTC in `bex-canary` (`tea-daif693dqjvc73e7as3g`) as the QA user over REST, with four free native web services, `autoDeploy: no`. Every fixture builds from `bex-co/bex` branch `main`, root directory `examples/default-env-probe` (unchanged since `c0cb02045`), and its build command starts with `sh probe.sh build`. Instead of committing new examples, the Passenger and Gunicorn build commands write their app files themselves (the deviation from § Blocked on the release gate):

| Fixture | Service id | Build command after `sh probe.sh build &&` | Start command |
| --- | --- | --- | --- |
| `passenger` (Ruby 3.4.12) | `srv-dbu2uuhc204s73bi6v8g` | writes `Gemfile` (`passenger` 6.1.0, `rack` 3.1.8) and `config.ru`; `BUNDLE_DEPLOYMENT=false bundle lock && bundle install && bundle exec passenger-config install-standalone-runtime --auto`; then `ruby -e 'require "minitest"; …'` prints `GEMCHECK` | `bundle exec passenger start` |
| `gunicorn` (Python) | `srv-dbu2uu84f0dc73876r10` | `pip install flask==3.1.1 gunicorn==23.0.0`; writes a Flask `app.py` and `gunicorn.conf.py` with `bind = "0.0.0.0:8000"` | `gunicorn app:app` |
| `py` (Python) | `srv-dbu2uug4f0dc73876r30` | — (w32/m425's probe fixture) | `sh probe.sh runtime && exec perl serve.pl` |
| `rb` (Ruby 3.4.12) | `srv-dbu2uuo4f0dc73876r50` | — (w32/m425's probe fixture) | `sh probe.sh runtime && exec perl serve.pl` |

- **DoD 1, fails, but not on this milestone's change.** The build log read `DEFAULT_ENV build PASSENGER_PORT=10000` and `PASSENGER_ENGINE=builtin`, and the pod's env carried the operator's `PASSENGER_PORT=10000`. Passenger never started: `Could not start the server engine: Daemon 'PassengerAgent watchdog' failed to start.`, exit 1, `CrashLoopBackOff`. Deploy `dep-dbu2uuhc204s73bi6v90` closed `update_failed` at 10:47:54 ("container exited shortly after start and is restarting repeatedly (last exit code 1)"). The pod runs uid 0 with every capability dropped (`CapEff: 0`, App `containerPolicy` empty, so strict). A local repro on `ruby:3.4` with the same Gemfile shows the cause. With Docker's default capabilities it answers `200` on 10000. With `--cap-drop ALL` it exits 1, and Passenger's log reads `Unable to lower Passenger watchdog's privilege to that of user 'root' and group 'root': cannot set supplementary groups: Operation not permitted (errno=1)`. With `--cap-drop ALL --cap-add SETGID` it answers `200` on 10000 again. This happens whatever the port, so it is a separate defect: [w32/m462](../../m462/README.md) (promoted from w32/038). The service's own `PASSENGER_PORT` case was not deployed, because no Passenger can start.
- **DoD 2 holds.**
  - Default: build and pod both read `GUNICORN_CMD_ARGS=--preload --access-logfile - --bind=0.0.0.0:10000`. Gunicorn logged `Listening at: http://0.0.0.0:10000`, and `/proc/net/tcp` showed only 10000 listening. The first rollout went `live` at 10:45:06, and `https://qa-20261117-m424-gunicorn.onbex.co/` answered `200`.
  - Its own `--bind=0.0.0.0:9000`: the pod carried no operator value. Gunicorn listened only on 9000, and deploy `dep-dbu31io4f0dc73876rag` went `live` (10:53:46) with probes on 9000, answering `200`.
  - Its own empty value: build `GUNICORN_CMD_ARGS=`, set and empty in the instance. Gunicorn listened only on the config file's 8000, App `status.detectedPort: 8000`, and deploy `dep-dbu34ipc204s73bi7030` went `live` (10:59:05), answering `200`.
- **DoD 3 holds.** `rb` read `GEM_PATH=/opt/render/project/.gems` at build and at runtime. In the instance, `GEM_HOME=/usr/local/lib/ruby/gems/3.4.0`, and `ruby -e 'require "minitest"; require "rake"'` succeeded (minitest 5.25.4, rake 13.2.1). `Gem.dir == Gem.default_dir` is `true`, and `Gem.path` is `/opt/render/project/.gems:/usr/local/lib/ruby/gems/3.4.0`. That is the first check of `GEM_HOME`'s expansion on the amd64 common base, and it passes. The Passenger build's `GEMCHECK` line read the same `GEM_PATH`, minitest 5.25.4 and `true`.
- **DoD 4 and 5** held on 2026-11-17 (§ Blocked on the release gate).
- **For w32/m425's t007 (the three ⏳ cells), read from the `py` and `rb` probe fixtures on this pin, 10:45–10:47 UTC:** Python `GUNICORN_CMD_ARGS=--preload --access-logfile - --bind=0.0.0.0:10000` at build and at runtime. Ruby `PASSENGER_PORT=10000` and `GEM_PATH=/opt/render/project/.gems` at build and at runtime. Both match Render's page. `PASSENGER_ENGINE=builtin` too, and neither runtime's variable reached the other: `py` read `PASSENGER_PORT` and `GEM_PATH` unset, and `rb` read `GUNICORN_CMD_ARGS` unset.
- **Cleanup:** all four services were deleted at 11:00 UTC, and each by-id `GET` answered `404`. The QA session was logged out.

## Blocked on w32/038 (2026-11-17)

**Gate: [w32/m462](../../m462/README.md) (promoted from w32/038) ships and reaches production.** Any `bundle exec passenger start` on a native Ruby service has to go live before DoD 1 can hold. Nothing in this milestone's code is at fault: `PASSENGER_PORT` reached the build and the pod as designed. Once a `chore(deploy): pin platform images to <sha>` commit contains w32/038's fix, and production's `bex-controller-manager` runs that digest, t007 reruns only DoD 1 with the `passenger` fixture above. That covers the default (live on its first rollout, listening on 10000, `200`), then the service's `PASSENGER_PORT=9000` (live on 9000, `200`). Then it deletes the fixture with by-id `GET` 404 evidence and closes the milestone. DoD 2–5 need no rerun.
