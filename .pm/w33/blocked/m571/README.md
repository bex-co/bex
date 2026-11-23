# w33 · m571 — A native build's image export takes less than half its 7 minutes

**Worker:** worker33 **Goal:** the time between a native build command's last line and `==> Build successful 🎉` on a 1.4 GB Node image falls from 7m09s–7m42s to under 3m30s. Each build records which part of its export the time went to, so the cut is chosen from measurements, and pulls and cache hits get no worse. **Status:** blocked (t001 done in `9fe2896295`; t002, the measurement, waits for a production pin carrying it)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Each native build records how long its image archive, layer-cache export and XDG/dependency archive take — **DONE** | 1h | — |
| [t002](t002.md) | Measure the split on a 1 GB+ Node image and choose the cut from the numbers | 1h30m | t001 |
| [t003](t003.md) | Ship the chosen cut | 1h | t002 |
| [t004](t004.md) | Simplify | 15m | t003 |
| [t005](t005.md) | Test coverage | 30m | t003 |
| [t006](t006.md) | Closeout (live check on production) | 40m | t004, t005 |

## Definition of done

On production, once the pin carries this milestone's commits, for a rebuild of `eden-dash-v3` (`srv-d9ndt8hmcglc739fkp50`, the first-party `bex` workspace), or a disposable `bex-canary` Node fixture with a runtime image of 1 GB or more:

1. From the build command's last line (`Done in …`) to `==> Build successful 🎉` takes under 3m30s. Today it is 7m54s (22:43:56 → 22:51:50 on 2026-11-22).
2. The release image pulls no slower than today: the `Pulling` → `Pulled` events of the release pod take 45 s or less (40 s on 2026-11-22).
3. The build after it still reuses what the cache holds: `cache-restore` reports a hit, and the dependency directories and `$XDG_CACHE_HOME` arrive.
4. The phase split that t001 records is in the milestone README, before and after.
5. `make test` (from `lego/operator/`) passes.

## Measurements

- **How to read the split (t001, 2026-11-22).** Once a pin carries t001, every native build whose buildkit phase succeeds leaves `export <part> <seconds>` lines in that phase's termination message: `image` (the OCI archive, BuildKit's count for the step), `layers` (the layers' compression, within `image`), `cache` (the layer cache's export), `xdg` (the `$XDG_CACHE_HOME` and dependency archive's solve) and `total` (from `==> Uploading build...` to the phase's end, all of them included). Three ways to read them, all read-only:
  - the operator's log line `build export seconds`, with `app`, `job` and `imageSeconds` … `totalSeconds` (`kubectl -n bex-system logs deploy/bex-controller-manager | grep 'build export seconds'`);
  - the build pod while its Job lives (one hour): `kubectl -n bex-build get pod -l job-name=<job> -o jsonpath='{.items[*].status.initContainerStatuses[?(@.name=="buildkit")].state.terminated.message}'`;
  - the operator's metrics, `bex_build_export_seconds{part}`.
- **Before (t002):** not yet measured.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w33` (the fixes found watching deploy `dep-dc1mrpt3vdpc73b0i0l0` of `eden-dash-v3` with the bex CLI, 2026-11-22), item 3; filed by `/pmx the fix to w33`. Re-checked against `origin/main` `e201eb552d`.
- **Evidence (production, read-only, 2026-11-22):**
  - The buildkit init container ran 22:34:49 → 22:51:04. The build command's output ended at 22:43:56, so 7m08s went to BuildKit's work after the command.
  - The push of the finished archive took 45 s and the release pod's pull 40 s. The runtime image is 1428 MB as the kubelet reports it.
  - The 2026-11-14 build of the same service spent 7m42s in the same stretch.
- **Where the time can go:** the OCI archive with gzip layers (`--output type=oci,dest=/output/image.tar`, `lego/operator/internal/build/build.go:1335`). The layer cache (`--export-cache type=local,…,mode=max,image-manifest=true,compression=zstd`, `:1353-1356`), whose `mode=max` exports the build command's layer, which a source change always invalidates because the step follows `COPY . .`. The XDG and dependency archive solve (`xdgcache.go:261-305`). Which one dominates is unmeasured: the build log and the container log both hide BuildKit's transcript (`nativeLogDisplay`, `build.go:202`).
- **Decision state:** changing the image's layer compression or the cache's `mode` revisits choices w7/m86 and ADR060 D3/D4 recorded. Whichever cut t002 picks is this proposal's recommendation, based on measurement and ADR008 (deploy speed is part of the product Render sets the bar for). It is not a user decision, and t002 records it as such in ADR060.
- **Goal linkage:** ADR008 (a Render-class deploy experience); ADR060 (in-cluster builds).
- **Expected outcome:** large native builds deploy about 4 minutes sooner, and the build pod frees its node-exclusive build slot sooner.
- **Why now:** this is the largest single part of a native Node deploy after the build command itself: 36% of this deploy. Every rebuild pays it, whatever the source change.
- **Render parity:** omitted. This is operator-internal mechanism with no REST, GraphQL, MCP or UI change, and the log lines stay as they are (w33/m570 owns them).
