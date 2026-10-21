# w5 · m144 — Native Go builds use Go's current release, Bun gains 1.4, and the freshness check notices the next release

**Worker:** worker5 **Goal:** a native Go service whose `go.mod` declares a current Go release builds, Bun's default matches Render's, and the next toolchain release is flagged instead of breaking builds silently. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Move the Go line to the current stable release — **DONE** | 40m | — |
| t002 | Execution gate: current and old `go.mod` versions build — **DONE** | 30m | t001 |
| t003 | Add Bun 1.4 as the default line — **DONE** | 30m | — |
| t004 | The freshness check flags a stale toolchain line — **DONE** | 45m | t001, t003 |
| t005 | ADR060 table and a dev-5 Go service — **DONE** | 30m | t002, t004 |
| t006 | Simplify — **DONE** | 20m | t005 |
| t007 | Test coverage — **DONE** | 45m | t005, t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

Total: **8 tasks, about 4h 10m**.

## Definition of done

- A native Go service declaring `go 1.26` builds and serves on dev-5.
- Bun 1.4 resolves by default.
- The freshness script flags a pin one Go minor behind upstream.

## Source + Goal linkage

- **Source:** third `/pm-brainstorm for w5` (2026-10-20), re-offered as the parity-only set and chosen by the user the same day. `golang:1.24-bookworm` sets `GOTOOLCHAIN=local`, so any `go.mod` with `go 1.25`+ fails ("go.mod requires go >= …"); Render builds Go with "latest stable". bex's own backend (`go 1.26.0`) and CLI (`go 1.27.0`) could not deploy as native Go services. The freshness workflow tracks same-tag digests, never newer lines.
- **Goal linkage:** GOAL.md #1/#3 (deploy, git push to deploy); ADR060 native builds; Render parity.
- **Expected outcome:** modern Go apps build on bex as they do on Render, and the next release is flagged.
- **Why now:** every Go module created since August 2025 defaults to a `go` directive bex refuses: a complete build failure for a language, not a degradation.
- **Render parity:** omitted: build toolchain only, no REST/GraphQL/MCP/UI change.

## Closeout (2026-10-20)

- The Go line is now `1.27`, the current stable release (`golang:1.27-bookworm`, pinned by digest in `lego/operator/internal/build/native.go`).
- `TestNativeGoBuildsCurrentAndOldModules` ran against real BuildKit (`BEX_BUILDKIT_INTEGRATION=1`, passed in 45s). Modules declaring `go 1.27` and `go 1.21` both build, and the image serves `go1.27.x`.
- Bun 1.4 (1.4.3) is the default line. 1.2 and 1.3 are still selectable (`nativeversion_test.go`).
- `scripts/lib/toolchain-freshness.py` marks a Go or Bun pin `changed` with `newer_line` when upstream has a newer major.minor (Go's JSON feed; GitHub's latest Bun release). Lines compare numerically, and a missing or malformed feed is only a warning (`scripts/build-toolchain-freshness.test.sh`).
- **Deviation from the DoD:** the "builds and serves on dev-5" check ran through the BuildKit execution gate above, not a dev-5 deploy. It uses the same generated Dockerfile, a real BuildKit build and a real container serving HTTP. dev-5 runs the operator image from main, so this pin reaches it only after the change ships.
