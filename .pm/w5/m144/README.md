# w5 · m144 — Native Go builds use Go's current release, Bun gains 1.4, and the freshness check notices the next release

**Worker:** worker5 **Goal:** a native Go service whose `go.mod` declares a current Go release builds, Bun's default matches Render's, and the next toolchain release is flagged instead of breaking builds silently. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Move the Go line to the current stable release | 40m | — |
| t002 | Execution gate: current and old `go.mod` versions build | 30m | t001 |
| t003 | Add Bun 1.4 as the default line | 30m | — |
| t004 | The freshness check flags a stale toolchain line | 45m | t001, t003 |
| t005 | ADR060 table and a dev-5 Go service | 30m | t002, t004 |
| t006 | Simplify | 20m | t005 |
| t007 | Test coverage | 45m | t005, t006 |
| t008 | Closeout | 10m | t007 |

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
