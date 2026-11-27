# w3 · m672 — Render's dated Node defaults 24.14.1 and 24.21.0 carry the CVE-2026-111497 Brotli fix

**Worker:** worker3 **Goal:** a no-signal Node service, or another runtime's companion Node, created 2026-04-21 to 2026-10-20 still builds on Render's exact 24.14.1 or 24.21.0, and that Node's bundled Brotli decoder rejects Node's own CVE-2026-111497 regression input. **Status:** blocked (t001–t004, t006 and t007 done; t005's production rebuild and t008 wait for a production deploy carrying `01af8d284d`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Extend the Node Brotli backport recipe to 24.14.1 and 24.21.0](done/t001.md) — **DONE** | 45m | — |
| t002 | [Publish the two runtime donors on amd64 and arm64](done/t002.md) — **DONE** | 1h30m | t001 |
| t003 | [Adopt the donors in native builds](done/t003.md) — **DONE** | 45m | t002 |
| t004 | [Guard the dated Node defaults and amend ADR060 and ADR018](done/t004.md) — **DONE** | 30m | t003 |
| t005 | [Verify on dev-3 and production](t005.md) | 30m | t004 |
| t006 | [Simplify](done/t006.md) — **DONE** | 15m | t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 30m | t005 |
| t008 | [Closeout](t008.md) | 10m | t007 |

## Definition of done

1. A native Node service with no version signal created on a date in Render's 24.14.1 or 24.21.0 row, and a Python, Ruby, Go, Rust or Elixir service's companion Node created on such a date, build on that exact release. The log still says `==> Using Node.js version 24.14.1 (default)` or `24.21.0 (default)`.
2. That Node passes `deploy/native-base/node-brotli/check.js` at build and at runtime (the malformed block-switch input fails with -17 in both decoders, and a valid stream round-trips). `process.versions` is unchanged apart from the repaired decoder: node, modules, napi, v8, uv, openssl, zlib, icu, and brotli is still `1.2.0`, as on Node's fixed 24.22.1. npm's version is unchanged too.
3. A test fails when any step of `exactDefaults[nativeNodeRuntime]` (`lego/operator/internal/build/nativeversion.go`) falls below its line's first Brotli-fixed release and has no reviewed backport donor in `nativeRuntimeArtifacts`.
4. ADR060's D7 addendum and ADR018's Runtime row (`docs/ADR018/services-lifecycle.md`) record the two releases as platform backports under D1=A. No divergence is recorded, because the versions are Render's.
5. On production, after a pin carrying t003, the bex workspace's Node web service created 2026-08-02 (m636 t007's fixture) rebuilds with `24.14.1 (default)`, passes the probe and serves 200.

## Source + Goal linkage

- **Source:** promoted from [w3/131](../../done/131.md) on 2026-11-26 by `/loopx w3`. The note asked the user to choose between (a) keeping Render's exact rows unpatched and (b) flooring both defaults at 24.22.1, which would be a divergence needing sign-off. Triage found a third option that existing decisions already cover. It keeps Render's exact versions and backports the decoder fix into them:
  - [ADR060's D7 addendum](../../../../docs/ADR060-build-worker-reliability-and-performance.md), user decision D1=A (2026-10-29), "accepts platform maintenance of security patches for frozen native runtime images while preserving the user's selected language version".
  - [w8/222](../../../w8/done/222.md), user decision D8 (2026-10-29), settled the same question for a dated default: Python 3.14.8 stayed Render's exact default and its Brotli was repaired on the shared base.
  - The recipe exists. `deploy/native-base/node-brotli/patch.sh` already rebuilds Node 14/16/18/20 with the fix while keeping their versions, and `nativeRuntimeArtifacts` (`lego/operator/internal/build/nativebases.go`) serves those trees by exact upstream identity.
  - So no new user decision is needed, and neither (a) nor (b) ships.
- **Evidence (2026-11-26):**
  - `node -p process.versions.brotli` prints `1.2.0` on `node:24.14.1-bookworm-slim`, `node:24.21.0-bookworm-slim` and `node:24.22.1-bookworm-slim`. `zlib.constants.BROTLI_DECODER_ERROR_FORMAT_BLOCK_SWITCH` is undefined on the first two and `-17` on 24.22.1.
  - Node's `deps/brotli` history: v24.14.1 and v24.21.0 both end at `a9a6a4cdb252` (cherry-pick of google/brotli `e230f474b87`, on `3a2de1c23b5a`, "deps: update brotli to 1.2.0"). v24.22.1 adds `49dff6f57aa7` ("deps: brotli: cherry-pick 0ed4268a4401abcb44720963"; `c/dec/decode.c` +58/−29, `c/include/brotli/decode.h` +1/−3) and `b5f41529df` (the `node_zlib.cc` constant and its test).
  - So Google's 1.0.10 and 1.1.1 archives, which patch.sh uses for the EOL lines, do not fit the 24 line. Its recipe applies `49dff6f57aa7` to Node's own 1.2.0 tree.
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first), because the versions stay Render's, together with ADR060 D7's platform security maintenance.
- **Expected outcome:** the services w3/m636 moves onto Render's older Node rows build on a decoder without the known out-of-bounds write, and the build log is unchanged.
- **Why now:** w3/m636's `41ff57632f` is on main and reaches production with the next pin. Until this milestone's adoption commit (t003) is on a production pin as well, the next build of any such service runs the unpatched decoder. m636 t001 counted at most four Node services and one Go service's companion Node on production. `3c57c4d2f5` moved only the qualification case off the band; the product is unchanged.
- **Why no existing gate catches it:** a production build runs `node-check.js` only for the four backported EOL lines (`commonNodeBackport`, `lego/operator/internal/build/nativecommon.go`). A modern line's dated default was assumed to be a fixed patch, which held until m636.
- **Render parity omitted:** no REST, GraphQL, MCP or dashboard change. The build-log version lines stay Render's.
- **Out of scope:** another exact 24.x patch below 24.22.1 that a tenant selects with `NODE_VERSION` keeps ADR060's rule for explicitly selected patches (the official image, no repair claim). Only the two dated defaults get donors here. The donor map is keyed by image identity, so `NODE_VERSION=24.14.1` and `NODE_VERSION=24.21.0` receive the same repaired trees.

## Progress — 2026-11-26

- **Shipped:** `410850e802` (t001: recipe, Bookworm compiler stage, publication matrix) and `01af8d284d` (t002–t004: both donors adopted, build-time probe and source record, guard, ADR060 and ADR018).
- **Donors:** 24.14.1 → `ghcr.io/bex-co/bex-cnb-run@sha256:210786383693b54c7e0044d0dfa7d5351bc0faebc686da451f08378e5d4f89a0` (run 45887397538); 24.21.0 → `…@sha256:970bf4c14ac3d44025c08246d00f1dd99d30274ad0e7489047eca256b39c32ba` (run 45887400787). Each passes `check.js` where the official binary fails, and keeps every `process.versions` key and npm.
- **Qualified:** families run [45910181919](https://github.com/bex-co/bex/actions/runs/45910181919) passed all 48 jobs, the new `node-exact24141`, `ruby-node-exact24141`, `node-exact24210` and `ruby-node-exact24210` cells included.
- **DoD:** 2–4 hold in code, on CI and in the ADRs. Item 1's log line is unchanged by construction (`TestNodeDefaultFollowsRendersDatedRows`). Item 5 waits for production: production pin `410850e80281` already carries w3/m636's `41ff57632f` but not the adoption, so until a pin carries `01af8d284d`, a rebuild in the moved bands still runs the official, unrepaired 24.14.1 or 24.21.0.
- **Next:** once `python3 scripts/loopx-guard.py board w3` lists this item as ready, run t005 step 2 (m636 t007's fixture rebuild can be the same deploy), record it in `t005.md`, then t008 closes the milestone.
