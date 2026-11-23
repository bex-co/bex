# w33 · m570 — A native build's log says `==> Uploading build...` when its build command ends, and `Uploaded in` covers the whole export

**Worker:** worker33 **Goal:** the line after a native build command's last output is Render's `==> Uploading build...`, printed as BuildKit starts writing the image. The minutes BuildKit spends exporting and compressing (7 min on a 1.4 GB Node image) run under that line instead of in silence, and `==> Uploaded in Ns` counts them. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | The buildkit phase prints `==> Uploading build...` when the build command's step ends and the export begins, and stamps the start for the push phase — **DONE** | 1h | — |
| [t002](done/t002.md) | The push phase prints no second `Uploading` line, counts `Uploaded in` from the stamp, and an export that fails says `==> Uploading build failed` — **DONE** | 45m | t001 |
| [t003](done/t003.md) | The live-deploy-following contract and ADR018's Build logs row say where the line sits — **DONE** | 30m | t002 |
| [t004](done/t004.md) | Render parity — **DONE** | 20m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004 |
| [t007](done/t007.md) | Closeout (live check on production) — **DONE** | 40m | t005, t006 |

## Definition of done

On production, once the pin carries this milestone's commits, for a native Node build (the next build of `eden-dash-v3`, `srv-d9ndt8hmcglc739fkp50` in the first-party `bex` workspace, or a disposable `bex-canary` fixture):

1. `bex logs -r <srv> --type build -o text`, read through every page, shows `==> Uploading build...` as the first line after the build command's output, timestamped within 5 s of that output's last line.
2. `==> Uploaded in Ns` is followed by `==> Build successful 🎉`, and `N` is within 5 s of the time between the two lines.
3. The dashboard deploy page and `bex logs --tail` during the build show the line at the same moment.
4. A build whose export fails (operator test, with the buildkit phase failing after the command step) ends with `==> Uploading build failed` and no `Build successful`.
5. `make test` (from `lego/operator/`) passes.

## Implementation record (2026-11-22, t001–t006)

- **The line:** the BuildKit container's display filter (`nativeLogDisplay`, `lego/operator/internal/build/build.go`) prints `==> Uploading build...` at the command step's `DONE` or `CACHED` line, after writing that moment to `/output/bex-upload-started` (`uploadStartedFile`). It prints the line only when the write succeeded. Everything after it runs under it: the dependency save, the secret-file links, the image's export and compression, the layer cache's export and the `$XDG_CACHE_HOME` archive.
- **The count:** the push phase (`nativePushNarration`) reads the stamp. With a number there it prints no line of its own and counts `==> Uploaded in Ns` from it. Without one it prints the line and counts from its own start, as before.
- **A failure after the line:** BuildKit's `#N` and `error:` lines stay out, and the filter ends the log with `==> Uploading build failed` whenever the phase did not exit 0, even when BuildKit said nothing. The deploy's reason still quotes BuildKit's error through the termination message, under the `build environment` step.
- **Docs:** ADR018 Build logs row (new w33/m570 entry; the w4/m324 entry amended); `docs/render-artifacts/live-deploy-following.md` § A native build's own lines.
- **Tests:** `TestNativeBuildLogSaysUploadingOnceTheCommandsStepEnds`, `TestNativeBuildSaysUploadingBeforeTheExportEnds`, `TestNativeBuildLogSaysAnUploadThatFailsAfterTheCommand`, `TestNativeBuildLogLeavesTheUploadLineToThePushWithoutItsMoment`, `TestNativeBuildPushCountsTheUploadFromTheBuildkitPhase`, `TestANativeBuildSaysUploadingOnce`; `TestNativeBuildLogShowsTheCommandAndItsOutput`, `TestNativeBuildLogSaysACachedCommandWasReused` and `TestNativeBuildLogNamesAFailedCommand` updated.
- **Not done here:** DoD items 1–3 needed a live build on production (now verified in t007 below).

## Unblock (2026-11-23)

**Source: verified evidence (loopx w33), no user decision involved. Affects t007.** The release gate cleared: the newest `chore(deploy): pin platform images to <sha>` commit on `origin/main` is `e6c65f5c62` (`… to 1a5a8e872398`, 2026-11-23 04:25 UTC), `git merge-base --is-ancestor 9abd5507f5 1a5a8e872398` exits 0, and production's `bex-controller-manager` and `bex-api` Deployments run its pinned digest (`sha256:65ef4f0d90b1492ae50edcc26368c37c81976badd99a38f74ebae345e185247a`).

## Production replay (2026-11-23, t007)

Pin `1a5a8e872398` (carries `9abd5507f5`; dashboard pin `sha256:5829a2c3945e85cd0811f623b5268717a8348f905ed79c2b98f7c64aad5dfffa`), QA user via `scripts/qa-login.sh` (revoked afterwards) and a temporary `bex-canary` API key (revoked afterwards), workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`). Disposable Free native Node web service `qa-20261123-m570-node` (`srv-dc21r0fqbg1c739j1ngg`) from `https://github.com/render-examples/express-hello-world` (`main`, `autoDeploy: "no"`), whose build command ran `npm install && dd if=/dev/urandom of=payload.bin bs=1M count=250 status=none && echo BUILD_CMD_DONE_MARKER`, so BuildKit spent 27 s exporting/compressing the ~250 MB image payload before `push` started.

1. **Build (`dep-dc21r0fqbg1c739j1nh0`, pod `bld-tea-daif693dqjvc73e7as3g-qa-20261123-m570-node-gen-1-8xfld`):**
   - Container phases: `buildkit` ran `11:03:51Z` → `11:05:07Z` (exit 0; `export image 20.4`, `export layers 14.6`, `export cache 5.4`, `export xdg 1`, `export total 27`); `push` ran `11:05:08Z` → `11:05:40Z` (exit 0, 32 s). Deploy reached `live` at `11:06:16.987Z`.
   - **Full build log paged forward (`bex logs -r srv-dc21r0fqbg1c739j1ngg --type build --direction forward` across `11:03:29.654239Z..11:06:16.987148Z`, 24 entries in 1 page, verified in both `-o json` and `-o text`, DoD 1 & 2):**
     - `11:04:40.542344525Z` (`11:04:40` in `-o text`): `BUILD_CMD_DONE_MARKER` (last line of the build command's output)
     - `11:04:40.676525Z` (`11:04:40` in `-o text`): `==> Uploading build...` — first line after the build command's output, **0.13 s** after it (within 5 s), and **27 s before** `buildkit` exited and `push` started (`11:05:08Z`), with no second `Uploading` line from `push`.
     - `11:05:40.73839433Z` (`11:05:40` in `-o text`): `==> Uploaded in 60s` — elapsed time between the two lines is **60.06 s** (27 s BuildKit export + 32 s push), matching `60s` within **0.06 s** (within 5 s).
     - `11:05:40.738444955Z` (`11:05:40` in `-o text`): `==> Build successful 🎉` — immediately following `==> Uploaded in 60s`.
   - **Live tail vs dashboard deploy page during the build (DoD 3):**
     - `bex logs -r srv-dc21r0fqbg1c739j1ngg --type build --tail -o text` received `==> Uploading build...` at wall-clock `11:04:41.629Z`, and `==> Uploaded in 60s` / `==> Build successful 🎉` at `11:05:42.142Z`.
     - Dashboard deploy page (`/services/srv-dc21r0fqbg1c739j1ngg/deploys/dep-dc21r0fqbg1c739j1nh0`, live in headless Chromium) rendered `==> Uploading build...` at wall-clock `11:04:42.099Z` (**0.47 s** from `bex logs --tail`, 26 s before `push` started) and `==> Uploaded in 60s` / `==> Build successful 🎉` at `11:05:41.591Z` (**0.55 s** from `bex logs --tail`).
2. **Cleanup:** `DELETE /v1/services/srv-dc21r0fqbg1c739j1ngg` returned 204; by-id `GET` answered 404. Temporary API key deleted (`DELETE` 204) and QA session revoked (`qa-login.sh --logout`).
3. **Tests (DoD 4 & 5):** `LC_ALL=C make test` (from `lego/operator/`) passes, including `TestNativeBuildLogSaysAnUploadThatFailsAfterTheCommand` (`internal/build`, 11.3 s) and the full `internal/controller` envtest suite.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w33` (the fixes found watching deploy `dep-dc1mrpt3vdpc73b0i0l0` of `eden-dash-v3` with the bex CLI, 2026-11-22), item 2; filed by `/pmx the fix to w33`. Re-checked against `origin/main` `e201eb552d`.
- **Evidence:**
  - The build log had `Done in 235.73s.` at 22:43:56, then nothing until `==> Uploading build...` at 22:51:05: 7m09s of silence. The 2026-11-14 build of the same service had 7m42s (09:10:29 → 09:18:11).
  - The buildkit init container ran until 22:51:04. The push itself took 45 s (`==> Uploaded in 45s`).
  - The silent stretch is BuildKit's work after the command: writing the OCI archive (`--output type=oci,dest=/output/image.tar`, `lego/operator/internal/build/build.go:1335`), the layer-cache export (`--export-cache … mode=max`, `:1353-1356`) and the XDG/dependency archive solve (`xdgcache.go`).
  - `nativeLogDisplay` (`build.go:202-275`) hides that transcript, and the push phase prints `==> Uploading build...` only when it starts (`nativePushNarration`, `build.go:310-314`, from w4/582).
- **Render reference:** in Render's deploy log, `==> Uploading build...` directly follows the build command's output, then `==> Uploaded …` and `==> Build successful 🎉` (render.com/tutorials/when-deploys-go-wrong/reading-render-logs, read 2026-11-22). Render prints nothing between the command's last line and the upload line. That page does not show whether Render's upload line also covers compression. Treating bex's export as part of the upload is this proposal's reading, not a verified Render fact.
- **Goal linkage:** ADR008 Render parity first; ADR018 "Build logs" row; `docs/render-artifacts/live-deploy-following.md`'s progress-line contract.
- **Expected outcome:** a user following a large native build sees it upload instead of a 7-minute pause that looks like a hang.
- **Why now:** it is the longest silence in a native deploy log: 36% of this deploy's 19m41s. w33/m571 will shorten it but not remove it.
- **Render parity:** included. The build log reaches `bex logs`, REST `/v1/logs`, the live tail and the dashboard deploy page.
