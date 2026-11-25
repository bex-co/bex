# w1 · m638 — Rust's latest stable reaches native Rust builds within about a day

**Worker:** worker1 **Goal:** a new stable Rust release (patch or minor) becomes native Rust builds' default toolchain within about a day with nobody's action, digest-pinned and smoke-tested, as Go's does since w4/719. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Plan a Rust bump | 45m | — |
| t002 | Smoke a candidate Rust image | 40m | — |
| t003 | Apply a Rust bump | 30m | t001 |
| t004 | Rust bump workflow | 45m | t002, t003 |
| t005 | Record the exception | 20m | t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 30m | t005 |
| t008 | Closeout | 10m | t007 |

## Definition of done

1. With a fixture release the planner answers `wait` while the official image lacks it on either architecture and `bump` once both carry it; live it answers `current`.
2. A simulated bump of the real tree passes `scripts/image-pin-validate.sh`, the inventory validation and `go test ./internal/build/...`; the smoke passes on the pinned image and fails for a newer `rust-version`.
3. The workflow is scheduled, listed in `scripts/ci-schedule-drift.sh`, and ADR060 D7 records the Rust exception.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one render aspect for w1` (2026-11-25), each claim re-checked by an independent verifier agent the same day before filing, item 3. Render ([rust-toolchain](https://render.com/docs/rust-toolchain)): "By default, Render uses the latest stable Rust toolchain." bex pins one Rust image (1.100.0) that moves only after the weekly freshness issue and a manual bump. That issue runs Thursdays at 06:17 UTC, before Rust's Thursday releases, so a release waits about a week before the issue opens (1.100.0: released 2026-11-12, pinned 2026-11-16 in w6/m291).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first), Render's "latest stable" default. "Within about a day" is bex's own target, taken from Render's Go changelog ("Automatic Go version updates", 2026-04-16); Render states no cadence for Rust.
- **Expected outcome:** a new stable Rust is the default for native Rust builds the day it is published in the official image.
- **Why now:** Rust 1.101.0 is due 2026-12-24 ([releases.rs](https://releases.rs/)); without this, it waits for a manual review over the holidays.
- **Render parity omitted:** no REST, GraphQL, MCP or dashboard change; build toolchain and CI only.
- **Out of scope:** the agent-session image's own Rust pin (`lego/agent-image/Dockerfile:18`); a Debian rebuild with the same Rust stays in the weekly review.
