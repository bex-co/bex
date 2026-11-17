# w32 · m425 — A production capture of Render's default environment variables, by service type, runtime and phase

**Worker:** worker32 **Goal:** ADR018's ✅ for default environment variables rests on a committed production capture of every variable Render documents, and the acceptance checks still owed for w8/272 and w8/202 are closed. **Status:** blocked — release gate, clears by itself: t001–t006 done. [t007](t007.md) fills the three cells w32/m424 changes once a pin contains `f1cdd052b`, then closes the milestone (§ Blocked).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Probe fixtures and capture plan — **DONE** | 30m | — |
| [t002](done/t002.md) | Deploy and capture on production — **DONE** | 75m | t001 |
| [t003](done/t003.md) | Record the capture as `docs/render-artifacts/default-env-live.md` — **DONE** | 30m | t002 |
| [t004](done/t004.md) | Fix the small mismatches, file the rest — **DONE** | 45m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004 |
| [t007](t007.md) | Closeout — **BLOCKED** (release gate) | 20m | t005, t006 |

## Definition of done

1. `docs/render-artifacts/default-env-live.md` records, for every variable render.com/docs/environment-variables lists, its observed presence and value at build and at runtime for each service type (web, private, worker, cron, static build) and source (native Node, Python, Ruby, Go, Rust, Elixir; Dockerfile build args; prebuilt image), with the production pin, the capture date and Render's fetch date. Each cell is ✅, ✖ with a fix commit or filed id, or — with a reason.
2. w8/272's owed acceptance (a Phoenix release with no clustering configuration boots with a long `node()`; `RELEASE_DISTRIBUTION=sname` set by the service keeps a short name) and w8/202's pending hosted acceptance are walked on production and recorded in ADR018.
3. Rows already proven on production reuse that evidence (w8/273, w8/274).
4. The ADR018 default-variables row links the capture.
5. Fixtures are deleted (by-id `GET` 404).

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for Render parity of render.com/docs/environment-variables, 2026-11-16, item 3; re-verified before filing. Correction from the proposal: w8/273 and w8/274 already passed on production, so this milestone reuses them.
- **Evidence:** no `docs/render-artifacts` capture exists for the default-variables row, while the env-var, secret-file and env-group rows each have one; w8/272 moved to `done/` with its production acceptance "Still owed" (`.pm/w8/done/272.md:48`); ADR018's `PORT` row says of w8/202 "Operator deployment and hosted acceptance remain pending".
- **Goal linkage:** ADR008 Render parity first; ADR018's ✅ backed by evidence rather than by unit, kind and local-build checks alone.
- **Expected outcome:** a committed matrix that any later default-variable change can be checked against, and two owed acceptance checks closed.
- **Why now:** default-variable changes shipped between 2026-11-11 and 2026-11-14 (w8/256, w8/272, w8/273, w8/274, w4/386, w21/076), and w32/m424 adds three more. Run it after w32/m424's code is on production so the capture sees those three; if m424 is still waiting on its production pin, mark those cells pending rather than waiting.
- **Render parity:** omitted: a verification milestone whose fixes are runtime-environment only. If t004 changes a REST, GraphQL, MCP or dashboard surface, add the Render parity task with `/pm add-task`.

## Triage (2026-11-17, `/loopx w32`)

**Work now; the m424 cells wait.** The production pin is `55c120858af0` (`9c79201b4`), which does not contain w32/m424's `f1cdd052b`. The next batched deploy builds main only once that pin is 6 h old (`deploy.yml`, ~13:47Z), or on a dispatch. Between `55c120858af0` and `main` only `f1cdd052b` touches `lego/types/renderenv`, `lego/operator/internal/build` or `lego/operator/internal/controller`, so the capture differs only in the cells m424 changes: Python's `GUNICORN_CMD_ARGS` and Ruby's `PASSENGER_PORT` and `GEM_PATH`. As this README's Why-now says, those cells are captured as the current pin has them and marked pending. Every other cell, w8/272's owed acceptance and w8/202's are not gated, so t001–t006 run now. The pending cells are filled from m424's own production walk (t007 there checks exactly those three on production), and this milestone's closeout waits for it.

## Capture plan (t001)

**Probe.** [`examples/default-env-probe`](../../../../examples/default-env-probe/README.md): `probe.sh` prints, by name only, the 43 variables render.com/docs/environment-variables lists (15 for every runtime; Elixir 2, Go 2, Node 2, Python 5, Ruby 13, Rust 4) plus `PORT`, as `DEFAULT_ENV <phase> NAME=value` or `… NAME unset`. It runs first in each build command and each start command, so a fixture prints the whole list in both phases, and a runtime's variables showing up on another runtime would show too. None of the names is a secret, and the fixtures set no variables of their own (the Elixir one's throwaway `SECRET_KEY_BASE` is never printed).

**Fixtures** (`bex-canary`, prefix `qa-20261117-m425-`, Auto-Deploy off, free where the type allows):

| Fixture | Source | Type | Captures |
| --- | --- | --- | --- |
| `node` | native Node | web (free) | every-runtime and Node cells, build and runtime; w8/202 (listens on 3000, `PORT` stays 10000) |
| `py` | native Python | web (free) | Python cells (`GUNICORN_CMD_ARGS` pending m424) |
| `rb` | native Ruby | web (free) | Ruby cells (`PASSENGER_PORT`, `GEM_PATH` pending m424) |
| `go` | native Go | web (free) | Go cells |
| `rs` | native Rust | web (free) | Rust cells (w8/273 reused for a service's own `ROCKET_PORT`) |
| `ex` | native Elixir, `render-examples/phoenix_hello` | web (free) | Elixir cells; w8/272: `node()` long by default, short with the service's `RELEASE_DISTRIBUTION=sname` |
| `dk` | Dockerfile | web (free) | which defaults reach a Docker build as build args, and the container's runtime set |
| `img` | prebuilt `busybox:1.37.0` | web (free) | runtime set of an image service (no build) |
| `ps` | native Node | private (starter) | the type cells for `pserv` |
| `wk` | native Node | worker (starter) | the type cells for `worker` |
| `cr` | native Node | cron (starter, one triggered run) | the type cells for `cron` |
| `st` | native static build | static (free) | the type cells for `static` (build only) |

**Cells.** Each variable × phase (build, runtime) for every fixture. The every-runtime set is compared on all twelve; a runtime's own defaults on its web fixture and, for Node, on the private, worker, cron and static fixtures too. A type-dependent variable (`RENDER_SERVICE_TYPE`, `RENDER_EXTERNAL_HOSTNAME`/`_URL`, `RENDER_DISCOVERY_SERVICE`, `RENDER_WEB_CONCURRENCY`, `WEB_CONCURRENCY`, `PORT`) is compared across the five types. Reused rather than redone: w8/273 (a service's own `ROCKET_PORT` wins, sweep `81af`) and w8/274 (Python's virtualenv, sweep `11ae`).

**Read back** with `GET /v1/logs?resource=<id>&type=build|app&text=DEFAULT_ENV` as the QA user; Elixir's `node()` with `bin/phoenix_hello rpc` in the instance (`kubectl exec` on the control plane). Every fixture is deleted afterwards with a by-id `GET` 404.

## Capture (t002–t004, 2026-11-17)

Recorded in [default-env-live.md](../../../../docs/render-artifacts/default-env-live.md): pin `55c120858af0`, 09:23–09:42 UTC, twelve fixtures, all deleted with by-id `GET` 404.

- **DoD 1:** every cell is ✅, — with a reason, ✖ `NODE_MODULES_CACHE` (w8/m161), or ⏳ for the three cells w32/m424 changes: Python's `GUNICORN_CMD_ARGS` read `--preload --access-logfile -`, and Ruby's `PASSENGER_PORT` and `GEM_PATH` were unset on this pin. No runtime's defaults reached another runtime, and the Docker build saw every declared default as a build arg.
- **DoD 2:** w8/272 passes (long `node()` by default, `phoenix_hello@<pod>` with the service's `RELEASE_DISTRIBUTION=sname`). w8/202 passes again (a Node process on 3000 went live on 3000 with `PORT=10000`). Both are recorded in ADR018.
- **DoD 3:** Rust's own `ROCKET_PORT` (w8/273) and the Python virtualenv (w8/274) cite their production checks.
- **DoD 4:** ADR018's default-variables row links the capture.
- **DoD 5:** every fixture is deleted (`GET` 404).
- **Found on the way:** port detection adopts a Phoenix release's `epmd` (4369) on the Free plan, so the service answers 502: w32/037.

## Blocked (2026-11-17)

**Release gate (clears by itself; no decision needed).** Only the three ⏳ cells remain, and they wait for the pin that carries w32/m424's `f1cdd052b`. Production runs the operator image of the newest `chore(deploy): pin platform images to <sha>` commit on `main`; that was `55c120858af0` here, which does not contain it. Check:

```bash
git fetch origin main
pin=$(git log origin/main --grep='^chore(deploy): pin platform images to' -1 --format=%s | sed -E 's/.*images to ([0-9a-f]+).*/\1/')
git merge-base --is-ancestor f1cdd052b "$pin" && echo "pinned: fill the three cells"
```

Then [t007](t007.md) reads the three variables on production: reuse w32/m424 t007's walk if it already recorded them, or recreate the `py` and `rb` fixtures from [`examples/default-env-probe`](../../../../examples/default-env-probe/README.md) as the QA user and read `GUNICORN_CMD_ARGS`, `PASSENGER_PORT` and `GEM_PATH` at build and runtime (expected: Render's values, with `--bind=0.0.0.0:10000`). It replaces the ⏳ cells in `default-env-live.md` and the sentence about them in ADR018's default-variables row, deletes the fixtures (by-id `GET` 404) and closes the milestone.
