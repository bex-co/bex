# w3 · m638 — Rust's latest stable reaches native Rust builds within about a day

**Worker:** worker3 **Goal:** a new stable Rust release (patch or minor) becomes native Rust builds' default toolchain within about a day with nobody's action, digest-pinned and smoke-tested, as Go's does since w4/719. **Status:** done (2026-11-26)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Plan a Rust bump](done/t001.md) — **DONE** | 45m | — |
| t002 | [Smoke a candidate Rust image](done/t002.md) — **DONE** | 40m | — |
| t003 | [Apply a Rust bump](done/t003.md) — **DONE** | 30m | t001 |
| t004 | [Rust bump workflow](done/t004.md) — **DONE** | 45m | t002, t003 |
| t005 | [Record the exception](done/t005.md) — **DONE** | 20m | t004 |
| t006 | [Simplify](done/t006.md) — **DONE** | 15m | t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 30m | t005 |
| t008 | [Closeout](done/t008.md) — **DONE** | 10m | t007 |

## Definition of done

1. With a fixture release the planner answers `wait` while the official image lacks it on either architecture and `bump` once both carry it; live it answers `current`.
2. A simulated bump of the real tree passes `scripts/image-pin-validate.sh`, the inventory validation and `go test ./internal/build/...`; the smoke passes on the pinned image and fails for a newer `rust-version`.
3. The workflow is scheduled, listed in `scripts/ci-schedule-drift.sh`, and ADR060 D7 records the Rust exception.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one render aspect for w1` (2026-11-25), each claim re-checked by an independent verifier agent the same day before filing, item 3. Render ([rust-toolchain](https://render.com/docs/rust-toolchain)): "By default, Render uses the latest stable Rust toolchain." bex pins one Rust image (1.100.0) that moves only after the weekly freshness issue and a manual bump. That issue runs Thursdays at 06:17 UTC, before Rust's Thursday releases, so a release waits about a week before the issue opens (1.100.0: released 2026-11-12, pinned 2026-11-16 in w6/m291).
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first), Render's "latest stable" default. "Within about a day" is bex's own target, taken from Render's Go changelog ("Automatic Go version updates", 2026-04-16); Render states no cadence for Rust.
- **Expected outcome:** a new stable Rust is the default for native Rust builds the day it is published in the official image.
- **Why now:** Rust 1.101.0 is due 2026-12-24 ([releases.rs](https://releases.rs/)); without this, it waits for a manual review over the holidays.
- **Render parity omitted:** no REST, GraphQL, MCP or dashboard change; build toolchain and CI only.
- **Out of scope:** the agent-session image's own Rust pin (`lego/agent-image/Dockerfile:18`); a Debian rebuild with the same Rust stays in the weekly review.

## Outcome (2026-11-26, done)

- **Mechanism.** `.github/workflows/rust-line-bump.yml` runs every three hours. `toolchain-freshness.py rust-release` compares the `[pkg.rust]` release of static.rust-lang.org's stable channel with the `RUST_VERSION` the pinned image sets on linux/amd64 and linux/arm64.
  - Once the official `rust:1-bookworm` index carries the new release on both, `scripts/rust-line-smoke.sh` runs on an arm64 and an amd64 runner with the network off. It checks that `rustc` is the release and that rustup's default is that release, then builds a crate declaring it as its `rust-version`.
  - `rust-apply` then rewrites only the line's `version` and digest in `native.go`, plus the `native-rust` inventory entry. The inventory validation, `scripts/image-pin-validate.sh` and the operator's `internal/build` tests run, and the commit is pushed to main as `github-actions[bot]`.
  - Go and Rust share one planner and applier. [ADR060 D7](../../../../docs/ADR060-build-worker-reliability-and-performance.md) records the exception.
- **Found on the way.** The applier re-encoded every other inventory entry's non-ASCII as `\u` escapes, which Go's `go-apply` also did. It is fixed, and both test fixtures check the bytes.
- **Live checks.** `rust-release` answers `current` (1.100.0), both locally and in [run 45920876294](https://github.com/bex-co/bex/actions/runs/45920876294) on `a698dce16a`. The first real bump waits for Rust 1.100.1 or 1.101.0, which is due 2026-12-24.
