# w4 · m37 — `bex upgrade` self-update command (update-channel phase 2)

**Worker:** worker4 **Goal:** raw-binary bex CLI installs can update themselves safely (`bex upgrade`), verified against signed `bex-cli/v*` release assets, without clobbering package-manager-owned files **Status:** done

## Tasks (in order)

| id   | title                                                           | est | depends_on               |
| ---- | --------------------------------------------------------------- | --- | ------------------------ |
| t001 | `bex upgrade` command + target-release/asset resolution         | 45m | — — **DONE**             |
| t002 | Download + atomic self-replace with rollback on failure         | 45m | w4/m37/t001 — **DONE**   |
| t003 | Verify asset: `checksums.txt` + cosign signature before replace | 45m | w4/m37/t002 — **DONE**   |
| t004 | Package-manager channel detection (brew Cellar → hint)          | 30m | w4/m37/t002 — **DONE**   |
| t005 | Simplify pass                                                   | 20m | w4/m37/t003, w4/m37/t004 — **DONE** |
| t006 | Test coverage                                                   | 30m | w4/m37/t003, w4/m37/t004 — **DONE** |
| t007 | Closeout                                                        | 10m | w4/m37/t006 — **DONE**   |

## Closeout (2026-08-16)

Shipped `bex upgrade` in the new `cli/internal/upgrade/` package, plus the `update`-package extension exposing release assets:

- **t001** — `bex upgrade` (+ `-n`/`--check`) resolves the newest `bex-cli/v*` release and its per-OS/arch archive by reusing `update.LatestRelease` (new, uncached; the passive-notice `Latest` now shares the same `fetchReleases`/`newest` internals). No-ops with "already up to date" when current; refuses on a `dev` build.
- **t002** — downloads the archive and replaces the running binary via stage-in-same-dir + atomic `os.Rename`; any earlier failure leaves the original untouched (implicit rollback), staged temp files always cleaned up.
- **t003** — verifies **in-process** (no `cosign` binary needed): the release's keyless sigstore signature over `checksums.txt`, pinned to the bex-cli release workflow's Fulcio identity (`sigstore-go`), then the archive's SHA-256 against that signed `checksums.txt`. Fail-closed on any error. The release pipeline already produced `checksums.txt.sigstore.json` (m35), verified present on the published `v0.1.0` — no `cli-release.yml` change needed.
- **t004** — a Homebrew/Linuxbrew-owned path prints `brew upgrade bex` instead of overwriting manager-owned files, before any network work.
- **t005** — `/simplify` (3 agents): collapsed `fetch()` onto `LatestRelease()`, extracted the `dev`/`""` sentinel into `update.IsReleaseBuild` (was triplicated), tightened per-download caps (256 MiB archive vs 1 MiB metadata) behind a shared `readCapped`.
- **t006** — 18 tests: asset resolution, version decision, bad-signature abort, checksum-mismatch abort, rollback on staging failure, brew short-circuit, dev refusal, missing-asset error, `readCapped` bound, and the `update` assets path.
- **Render parity task omitted** — CLI-only local command, no REST/GraphQL/MCP/dashboard surface (rationale in Source + Goal linkage).

**Verified:** `go test ./...`, upstream import pin, `go vet`, all 4 cross-compile targets, and a **live** `bex upgrade -n` against real GitHub correctly reporting `v0.0.1 → v0.1.0`. Docs updated (`docs/bex-cli.md`). Dependency cost accepted: `sigstore-go` adds ~15–18 MB to the binary (in-process keyless verification is worth it for a self-updater); upstream `render-oss/cli` pin + tests unaffected.

**Deferred (release-cadence, not an implementation gap):** the full live download→sigstore-verify→atomic-replace can only run once a release newer than the installed one exists — `v0.1.0` is currently the newest, so there is nothing to upgrade *to*. It is exercised end-to-end by unit tests and will be live-provable the first time `bex-cli/v0.2.0` ships.

## Definition of done

Running `bex upgrade` on a raw-binary install:

- resolves the newest `bex-cli/v*` GitHub release, and no-ops (with a clear "already latest" message) when the running `bexVersion` is current;
- when a newer release exists, downloads the per-OS/arch asset, **verifies it** against the release's `checksums.txt` and cosign signature, and only then atomically replaces the running binary — restoring the prior binary if any step fails;
- when the running binary lives under a package-manager-owned path (e.g. a Homebrew Cellar), refuses to overwrite and prints the `brew upgrade` hint instead;
- the whole flow is exercised by tests (asset selection, version comparison via the existing `update` package, verification-failure abort, brew-path short-circuit, rollback).

## Source + Goal linkage

- **Source:** inbox note `w4/030.md` (CLI update-channel research session, 2026-08-14), the documented **phase 2** of the update channel whose phase-1 passive notice shipped in **w4/m34** and whose release artifacts (`checksums.txt`, signed assets, Homebrew tap) shipped in **w4/m35**. User promoted it 2026-08-16.
- **Goal linkage:** distribution/DX for the bex CLI (the accepted CLI line of work — m34/m35; **not** a from-scratch CLI, which `DO_NOT_DO.md` #31 forbids). Closes the "raw-binary users are stuck until they manually re-download" gap that m34's passive notice only _reports_.
- **Expected outcome:** a raw-binary `bex` user runs `bex upgrade` and lands on the latest signed release in one step; Homebrew users are correctly deflected to `brew upgrade`.
- **Why now:** the gate recorded in `w4/030.md` ("promote once release cadence justifies it / m34 notice ships") is met — m34's notice and m35's signed-asset pipeline + tap are live, so every prerequisite (`bexVersion` injection, `cli/internal/update` release listing/filter, `checksums.txt`, cosign infra) already exists; this is purely the consuming command.
- **Render parity task omitted:** `bex upgrade` is a CLI-only local command with **no** REST/GraphQL/MCP/dashboard surface — nothing to keep consistent across bex-api surfaces or against render.com (Render's CLI has no equivalent bex-specific self-update). Standing closing tasks are therefore Simplify → Test coverage → Closeout only.
