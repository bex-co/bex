# w33 · m570 — A native build's log says `==> Uploading build...` when its build command ends, and `Uploaded in` covers the whole export

**Worker:** worker33 **Goal:** the line after a native build command's last output is Render's `==> Uploading build...`, printed as BuildKit starts writing the image. The minutes BuildKit spends exporting and compressing (7 min on a 1.4 GB Node image) run under that line instead of in silence, and `==> Uploaded in Ns` counts them. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | The buildkit phase prints `==> Uploading build...` when the build command's step ends and the export begins, and stamps the start for the push phase | 1h | — |
| [t002](t002.md) | The push phase prints no second `Uploading` line, counts `Uploaded in` from the stamp, and an export that fails says `==> Uploading build failed` | 45m | t001 |
| [t003](t003.md) | The live-deploy-following contract and ADR018's Build logs row say where the line sits | 30m | t002 |
| [t004](t004.md) | Render parity | 20m | t003 |
| [t005](t005.md) | Simplify | 15m | t004 |
| [t006](t006.md) | Test coverage | 30m | t004 |
| [t007](t007.md) | Closeout (live check on production) | 40m | t005, t006 |

## Definition of done

On production, once the pin carries this milestone's commits, for a native Node build (the next build of `eden-dash-v3`, `srv-d9ndt8hmcglc739fkp50` in the first-party `bex` workspace, or a disposable `bex-canary` fixture):

1. `bex logs -r <srv> --type build -o text`, read through every page, shows `==> Uploading build...` as the first line after the build command's output, timestamped within 5 s of that output's last line.
2. `==> Uploaded in Ns` is followed by `==> Build successful 🎉`, and `N` is within 5 s of the time between the two lines.
3. The dashboard deploy page and `bex logs --tail` during the build show the line at the same moment.
4. A build whose export fails (operator test, with the buildkit phase failing after the command step) ends with `==> Uploading build failed` and no `Build successful`.
5. `make test` (from `lego/operator/`) passes.

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
