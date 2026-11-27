# w3 · m636 — Services created before bex's default cutoffs build on Render's dated defaults

**Worker:** worker3 **Goal:** a native service with no version signal builds on the default Render's table gives its creation date, for every date since bex's first service, and no line-pin refresh moves it. **Status:** done (2026-11-27: on production pin `6ba91ca5b72f` a rebuild of the bex workspace's Node web service created 2026-08-02 logged `==> Using Node.js version 24.14.1 (default)` and serves 200)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Count the services each band moves on production](done/t001.md) — **DONE** | 30m | — |
| t002 | [Node: Render's 2026-04-21 and 2026-09-17 rows](done/t002.md) — **DONE** | 30m | t001 |
| t003 | [Ruby: Render's 2025-06-12 → 3.4.4 row](done/t003.md) — **DONE** | 25m | t001 |
| t004 | [Python: Render's 2026-02-11 band](done/t004.md) — **DONE** | 45m | t001 |
| t005 | [uv and Erlang/OTP: Render's dated tables](done/t005.md) — **DONE** | 40m | t001 |
| t006 | [Correct ADR018 and ADR060](done/t006.md) — **DONE** | 30m | t002, t003, t004, t005 |
| t007 | [Verify on dev-3 and production](done/t007.md) — **DONE** | 45m | t006 |
| t008 | [Simplify](done/t008.md) — **DONE** | 15m | t007 |
| t009 | [Test coverage](done/t009.md) — **DONE** | 30m | t007 |
| t010 | [Closeout](done/t010.md) — **DONE** | 10m | t009 |

## Definition of done

1. For Node (and the companion Node), Ruby, Python, uv and Elixir's Erlang/OTP, a service created on any date from 2026-06-05 resolves to the value Render's version page gives that date; tests cover each boundary.
2. A test that bumps each line's reviewed patch shows no dated service's default moves.
3. ADR018's Runtime row no longer carries the Python and Elixir residuals and says truthfully what a line bump still moves; ADR060's table bolds 3.14.
4. On production a rebuilt pre-cutoff first-party service logs Render's dated default and serves 200.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one render aspect for w1` (2026-11-25), each claim re-checked by an independent verifier agent the same day before filing, item 5. It was proposed as a decision note. Verification found the bands are recorded only as residuals with no user sign-off (w8/174 asked for one; ADR018's Runtime row and ADR060 record them, and the Node and uv bands are not in ADR018 at all), so under AGENTS.md ("Never park parity work on an optional decision. Ship the parity-sufficient choice") this milestone ships Render's behavior.
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first); the AGENTS.md rule that a divergence stands only when ADR018 records it with the user's sign-off.
- **Expected outcome:** a no-signal native service builds on the version Render gives a service created the same day, and stays there through every line refresh (today these services move whenever a line is bumped: Node 24.21.0 → 24.22.3, Python 3.13.15 → 3.13.18, Ruby 3.4.11 → 3.4.12 since October).
- **Why now:** every refresh moves these services again ([m635](../../done/m635/README.md) is about to move Ruby 3.4.12 → 3.4.13), and [m637](../../done/m637/README.md)'s whole-table comparison would otherwise need exceptions no one signed off.
- **Risk:** services created 2026-02-11 to 2026-10-23 with no Python signal move 3.13 → 3.14 on their next build, and Node/Ruby services move down to Render's older patches (24.14.1, 24.21.0, 3.4.4). That is what Render gives such services. Precedent: w4/620, w9/138 and w8/371 already moved existing services onto Render's dated defaults. Holding Python, or freezing all bands at today's patch, would be a divergence needing the user's sign-off in ADR018; it is not filed.
- **Render parity omitted:** no REST, GraphQL, MCP or dashboard change; build resolution and build-log lines only.
- **Out of scope:** a service of unknown creation time keeps the line's pin.

## Blast radius — 2026-11-26 (t001)

Read on production at origin/main `8c3be4c128`: all 27 App CRs through the admin kubeconfig (`scripts/fetch-app-kubeconfig.sh`), and each native service's newest `==> Using …` build line from Loki (`{type="build"}`, 30-day retention). Counts only. Ten Apps build from a Dockerfile, seven run a prebuilt image and three older `builder: auto` Apps take the Dockerfile path, so none of those resolve a native default. That leaves six Node services and one Go service:

| runtime | creation band | Render's default | services | newest build |
| --- | --- | --- | --- | --- |
| Node | 2026-06-05 to 2026-09-16 | 24.14.1 (was the 24 line's pin, 24.22.3) | 3 | 1 logged `24.22.3 (default)` on 2026-11-23; 2 have no build in the 30-day window (one is Failed), so whether they carry a signal was not read |
| Node | 2026-09-17 to 2026-10-20 | 24.21.0 (was 24.22.3) | 1, plus 1 Go service's companion Node | the Node service is suspended with no build in the window; the Go service last built 2026-10-30 |
| Node | from 2026-10-21 | 24.22.1 (unchanged) | 2 | both logged `24.22.1 (default)` |
| Ruby | 2026-06-05 to 2026-11-02 | 3.4.4 (was the 3.4 line's pin before 2026-10-23) | 0 | — |
| Python | 2026-06-05 to 2026-10-11 / 2026-10-12 to 2026-10-22 | 3.14.3 / 3.14.8 (was the 3.13 line's pin) | 0 | — |
| uv | 2026-06-05 to 2026-09-08 / 2026-09-09 to 2026-10-11 / 2026-10-12 to 2026-11-13 | 0.10.2 / 0.12.10 / 0.12.24 (was 0.12.10) | 0 | — |
| Elixir's Erlang/OTP | 2026-06-05 to 2026-11-15 | 28.0.2 (was the image's OTP) | 0 | — |

So on production the change moves at most four Node services and one companion Node on their next build; every Python, Ruby, uv and Elixir build Loki holds belongs to a QA service that has since been deleted.

## Implementation — 2026-11-26 (t002–t006, t008, t009)

Render's version pages, re-read 2026-11-26, match the rows this milestone was filed with; uv's page also has 0.10.2 from 2026-02-11, the row in force on 2026-06-05, so it is in the table too.

- **Tables** (`lego/operator/internal/build/nativeversion.go`, `lego/types/toolchains/python.go`): Node 24.14.1 from 2026-04-21, 24.21.0 from 2026-09-17, 24.22.1 from 2026-10-21 (the companion Node reads the same table); Ruby 3.4.4 from 2025-06-12; Python 3.14.3 from 2026-02-11, 3.14.8 from 2026-10-12, 3.14.10 from 2026-11-03, with the default line read from the dated step instead of a separate cutoff; uv 0.10.2 / 0.12.10 / 0.12.24 / 0.13.7 from 2026-02-11 / 09-09 / 10-12 / 11-03 (`uvDefaults`); Erlang/OTP 28.0.2 from 2025-06-12. bex's own cutoffs (`rubyDefaultCutoff`, `toolchains.PythonCreationCutoff`, `uvDefaultCutoff`, `erlangDefaultCutoff`) are gone; Render's shared dates are named once (`toolchains.RenderDefaults20260211`, `…20261012`, `…20261103`, and `renderDefaults20250612` in the operator).
- **Unknown age:** a build with a zero creation time keeps each line's pin (Python 3.13, uv 0.12.10), as before. Every App build has its creation time.
- **Agent sessions (t004):** they read the same Python table by the session's creation time (`BEX_AGENT_SESSION_CREATED_AT`, sent since w4/m544 on 2026-11-20). A session or snapshot build created from 2026-02-11 to 2026-10-22 with no Python signal now fetches 3.14.3 (or 3.14.8 from 2026-10-12) and falls back to the baked 3.14.10, where it ran the baked 3.13.18 before. One without a creation time keeps 3.13.18. `TestAgentToolchainOlderSessionGetsItsDatedDefault` pins it.
- **Docs (t006):** ADR018's Runtime row ([services-lifecycle.md](../../../../docs/ADR018/services-lifecycle.md)) drops the Python band from its divergences and the Erlang/OTP residual, names the Node, Ruby, Python, uv and Erlang/OTP rows, and says only a build of unknown creation time follows a line's pin; ADR060's D7 table bolds 3.14 and its no-signal and uv entries give the dated tables; ADR047 gives a session's older Python rows.
- **Tests (t009):** `TestNodeDefaultFollowsRendersDatedRows` (every Node boundary from 2026-06-05, primary and the companion of each non-Node runtime), `TestDatedDefaultsSurviveALinePinBump` (each runtime's default line bumped past every step: each step still resolves exactly, only an unknown age follows the pin, steps ascending and on the default line), `TestErlangAndUVDatedDefaultsAreWellFormed`, the Python boundaries in `TestResolvePython` and its pin-bump test (3.13 and 3.14 bumped together), and the Ruby, Python, uv and Erlang/OTP tests moved onto Render's dates. Mutation spot checks, each reverted: dropping the 24.21.0 row fails 4 Node subtests; moving Ruby's 3.4.4 back to 2026-10-23 fails `TestRubyDefaultIsRendersExactPatch`; making `defaultExact` honour only the newest step fails the pin-bump test for Node and Ruby; Erlang/OTP back to 2026-11-16 fails `TestDefaultElixirBuildsOnRendersDefaultOTP`; dropping uv's 2026-09-09 row or Python's 3.14.3 row fails their boundary cases. `make test`, `make lint`, `go test ./...` in `lego/types` and the agent-toolchain tests pass.
- **Simplify (t008):** `/simplify` over the diff shared the Erlang/OTP registry fixture between two tests, one date helper and one ascending-table check; nothing else to change.

## Verification — 2026-11-26 (t007, local part)

A backdated App cannot be created (the API server sets `creationTimestamp`), so the resolve-phase harness stood in: `ResolveNativeBuildExact` against Docker Hub with a creation time in each moved band, on the production recipe (the pinned common base). Each resolved Dockerfile pins the dated default by digest and checks it at build time:

| service created | log lines | pinned image |
| --- | --- | --- |
| Node, 2026-08-01 | `==> Using Node.js version 24.14.1 (default)` | `node:24.14.1-bookworm@sha256:80fc9349…`, `[ … = 24.14.1 ]` |
| Node, 2026-10-01 | `==> Using Node.js version 24.21.0 (default)` | `node:24.21.0-bookworm@sha256:3d27e5c1…` |
| Python with `uv.lock`, 2026-08-01 | `Python version 3.14.3 (default)`, `uv version 0.10.2 (default)`, companion `Node.js version 24.14.1 (default)` | `python:3.14.3-bookworm@sha256:084a6aca…`, `uv==0.10.2` |
| Python with `uv.lock`, 2026-10-15 | `Python version 3.14.8 (default)`, `uv version 0.12.24 (default)`, companion Node 24.21.0 | `python:3.14.8-bookworm@sha256:d9717827…`, `uv==0.12.24` |
| Ruby, 2026-08-01 | `==> Using Ruby version 3.4.4 (default)`, companion Node 24.14.1 | `ruby:3.4.4-bookworm@sha256:f7ab76e2…` |
| Elixir, 2026-08-01 | `==> Using Erlang version 28.0.2 (default)` before the Elixir line | `erlang:28.0.2.0@sha256:bf9213a7…` with hexpm's `1.18.4-erlang-28.0.2` build |

The BuildKit build of those Dockerfiles did not run here: Docker Desktop on this host times out pulling the common base from ghcr.io (`DeadlineExceeded`, and a plain `docker pull` of it never starts) while Docker Hub pulls work. Each exact-image swap is the path an explicit exact version already takes on production, so the real build is the production check that remains (t007 step 2, DoD 4).

## Unblock check — 2026-11-26 (t007 step 2 waits for `01af8d284d`)

- **Affected:** t007 step 2 and t010; w3/m672 t005 shares the same deploy.
- **Finding (verified evidence):** the newest production pin on origin/main, `918667a83f` ("pin platform images to `410850e80281`", 20:20Z), carries `41ff57632f` (`git merge-base --is-ancestor 41ff57632f 410850e80281` exits 0), so the gate written on 2026-11-26 holds. It does not carry `01af8d284d`, w3/m672's adoption of the Brotli-fixed donors for Node 24.14.1 and 24.21.0. Read through the admin kubeconfig at 00:35Z on 2026-11-27: the fixture, the bex workspace's Node web service created 2026-08-02, last built 2026-11-23 and has not built since the pin; the bex workspace's Go service created 2026-10-19 last built 2026-10-24. Nothing of bex's own has built on an unrepaired dated default yet.
- **Decision (worker, under w3/m672's security note):** t007 step 2 does not run on this pin. On `410850e80281` a manual deploy of the fixture moves it from Node 24.22.3, whose decoder is fixed, onto the official 24.14.1, whose bundled Brotli decoder fails Node's CVE-2026-111497 regression ([m672](../m672/README.md)'s evidence). It stays there until the next pin. This run would create that exposure on a first-party production service itself.
- **Why waiting costs nothing:**
  - Parity is unchanged. The version and the log line are Render's on both pins (`TestNodeDefaultFollowsRendersDatedRows`; m672 DoD 1).
  - The wait is short. Production pins landed at 00:25Z, 07:02Z, 13:52Z and 20:20Z on 2026-11-26, and `01af8d284d`'s deploy run 45910176679 succeeded.
  - `01af8d284d` descends from `41ff57632f`. One deploy after a pin carrying it is this check and m672 t005's at once, where deploying now would need a second deploy for m672.
- **Remaining gate:** ``pin must carry `01af8d284d` ``. It clears by itself and needs nobody.
- **Next:** deploy the fixture manually. Its build log reads `==> Using Node.js version 24.14.1 (default)` and the service answers 200 (t007, DoD 4). Run m672 t005's probe on the same deploy, then t010 closes this milestone.

## Unblock — 2026-11-27 (`/loopx w3`, gate cleared)

- **Affected:** t007 step 2 and t010. **Source:** verified evidence.
- **Finding:** the newest production pin on origin/main, `9c76d4ae72` ("pin platform images to `6ba91ca5b72f`", 03:12Z), carries `01af8d284d` and `41ff57632f` (`git merge-base --is-ancestor` exits 0 for both). Every `bex-system` Deployment on production runs `bex-operator@sha256:0ba1d0dc…`, the digest that pin writes, read through the admin kubeconfig at 03:20Z.
- **Next:** the production check below, on this pin.

## Verification — 2026-11-27 (t007 step 2, production, DoD 4)

The bex workspace's Node web service created 2026-08-02 (`srv-d9ndt8hmcglc739fkp50`, no version signal) got a manual deploy at the commit it was already running, `c7e28aa685`: `dep-dc4fg5a3u9qc73car67g`, triggered 03:25:09Z through `POST /v1/services/{id}/deploys` as the QA user (a member of the bex workspace), live 03:36:38Z.

| check | before (newest build, 2026-11-23) | after (`dep-dc4fg5a3u9qc73car67g`) |
| --- | --- | --- |
| build log (`GET /v1/logs`, `type=build`) | `==> Using Node.js version 24.22.3 (default)` | `==> Using Node.js version 24.14.1 (default)` |
| `node -v` in the live instance | not read | `v24.14.1` at `/usr/local/bin/node` |
| `/healthz` on its `.onbex.co` host, `/` there and on its custom domain | 200, 200, 200 (03:24Z) | 200, 200, 200 |

- The build took w3/m672's repaired donor. The live instance carries `/usr/share/bex-node-brotli/sources.txt` (`Node 24.14.1; source sha256:78225077…; Brotli 1.2.0; patch nodejs/node@49dff6f57aa7 sha256:1e9ee182…`), and `node /usr/share/bex-security/node-check.js` exits 0 there (the script is silent on a pass and asserts on a failure). The user-facing build log does not echo the Dockerfile's own `node-check.js` step, so this run read the build-phase probe only as a build that succeeded with that step in its recipe (`nativecommon.go`). m672 t005 records its own acceptance.
- The companion Node's 24.21.0 row was not rebuilt on production. The bex workspace's Go service created 2026-10-19 is the only first-party service in that band. Its builds log the companion Node, but it answers 404 at `/` and at `/healthz`, so it could not show the 200 this task asks for. The row's resolution and log line are covered by `TestNodeDefaultFollowsRendersDatedRows` and the resolve-phase run above.
- DoD 1 to 3 re-checked on main `bb842fd3b6`: `TestNodeDefaultFollowsRendersDatedRows`, `TestDatedDefaultsSurviveALinePinBump`, `TestErlangAndUVDatedDefaultsAreWellFormed`, `TestRubyDefaultIsRendersExactPatch`, `TestDatedNodeDefaultsHaveAFixedDecoder` and `TestResolvePython` pass, and ADR060's D7 table bolds 3.14.
- The QA session was logged out afterwards (then 401), and its jar was deleted.

## Closed — 2026-11-27 (`/loopx w3`)

Every task is done and the definition of done holds: DoD 1 to 3 by the tests and docs above, DoD 4 by the production rebuild. Production pin `6ba91ca5b72f`.
