# w3 · m635 — Build toolchain pins are fresh again and #201 closes

**Worker:** worker3 **Goal:** every pinned build-toolchain image equals its upstream at review time, the native Bun 1.4 and Ruby 3.3 / 3.4 / 4.0 lines carry the current patches, and the freshness monitor closes #201 on its own run. **Status:** blocked (t001–t005, t007, t008 done; t006 and t009 wait for a production deploy carrying `bef3a32083`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Re-resolve the drift and list what moves](done/t001.md) — **DONE** | 20m | — |
| t002 | [Bump the native Bun and Ruby lines](done/t002.md) — **DONE** | 45m | t001 |
| t003 | [Refresh the Paketo builder and stack images](done/t003.md) — **DONE** | 45m | t001 |
| t004 | [Adopt the rebuilt native-common-bookworm image](done/t004.md) — **DONE** | 30m | t001 |
| t005 | [Qualify the new images on dev-3](done/t005.md) — **DONE** | 60m | t002, t003, t004 |
| t006 | [Production check and #201 closes](t006.md) | 30m | t005 |
| t007 | [Simplify](done/t007.md) — **DONE** | 15m | t006 |
| t008 | [Test coverage](done/t008.md) — **DONE** | 30m | t006 |
| t009 | [Closeout](t009.md) | 10m | t008 |

## Definition of done

1. At closeout `python3 scripts/lib/toolchain-freshness.py resolve` reports no drift for the entries #201 listed (drift newer than the review opens its own issue).
2. #201 is closed by the scheduled freshness run, not by hand.
3. On production a `BUN_VERSION=1.4` build logs Bun 1.4.5 and a Ruby 3.4-line build logs the new patch, both serving 200, and the ClusterBuilder is Ready on the new builder.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one render aspect for w1` (2026-11-25), each claim re-checked by an independent verifier agent the same day before filing, item 1. GitHub issue #201, open since 2026-11-19 with no board owner. Precedent: [w6/m291](../../../w6/done/m291/README.md) (#105).
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first) and build supply-chain freshness ([ADR060 D7](../../../../docs/ADR060-build-worker-reliability-and-performance.md)). The parity case rests on Bun: Render's `BUN_VERSION=latest` "Uses the most recent stable release of Bun for every deploy", and a lockfile-only Bun service of unknown age builds on the 1.4 line's pin (`nativeversion_test.go:1014`), which is 1.4.4 while Bun 1.4.5 is out. Render's docs say nothing about how a partial Ruby version resolves, so no Ruby parity claim is made here.
- **Expected outcome:** native Bun and Ruby builds use current releases, the Paketo stack carries upstream fixes, and the review issue is closed.
- **Why now:** the issue has sat for six days with every observed digest already superseded; w9/138 deliberately left the 3.4 line on 3.4.12 until a real-build qualification, which this milestone runs.
- **Render parity omitted:** no REST, GraphQL, MCP or dashboard change; pins only, as in m291.

## Review list — 2026-11-26 (t001, `toolchain-freshness.py resolve` at `cb2e989e7d`)

`validate` passes. 25 of the 56 entries have moved upstream, #201's eight among them; #201's own digests were already stale. Versions were read from each image's linux/amd64 and linux/arm64 configs (the CNB builder is a single amd64 image):

| id | #201 | upstream | committed → observed | carries |
| --- | --- | --- | --- | --- |
| `cnb-builder` | yes | `docker.io/paketobuildpacks/builder-jammy-base:latest` | `5cd813b83496…` → `4ba599c03e4b…`, then `906ba1546346…` at 15:45Z | lifecycle 0.21.29 → 0.21.30, 135 buildpacks, stack `io.buildpacks.stacks.jammy` |
| `cnb-run-image` |  | `ghcr.io/bex-co/bex-cnb-run:latest` | `77061d60c38f…` → `c01de4f8624d…` | the same Paketo run base, republished by `cnb-run-image.yml` run 45019148491; rebuilt on the new `cnb-stack-run` instead |
| `cnb-stack-build` | yes | `docker.io/paketobuildpacks/build-jammy-base:latest` | `5fce29993757…` → `267bfd510aa3…`, then `763449c86cdd…` at 13:37Z | stack released 2026-11-26T00:51:36Z, then again at 12:43:57Z (was 2026-11-16) |
| `cnb-stack-run` | yes | `docker.io/paketobuildpacks/run-jammy-base:latest` | `6031575989ab…` → `64ca273de9e3…`, then `031b3fa694e7…` at 13:37Z | stack released 2026-11-26T00:51:36Z, then again at 12:43:57Z (was 2026-11-16) |
| `native-bun` | yes | `docker.io/oven/bun:1.4` | `1d22548f177f…` → `3aa040e5445e…` | Bun 1.4.4 → 1.4.5 |
| `native-common-bookworm` | yes | `ghcr.io/bex-co/bex-cnb-run:native-common-bookworm` | `49dbbd727e03…` → `d63d00b79187…` | the push run of `3624550656` (every `native*.go` push republishes the tag); adopted through a families run instead (t004) |
| `native-common-bookworm-source` |  | `docker.io/library/buildpack-deps:bookworm` | `f53cd4c58cdb…` → `c82cc01646f8…` | Debian bookworm root `@1793827419` → `@1795392000` |
| `native-elixir` |  | `docker.io/library/elixir:1.18` | `16382709b376…` → `6aff289bc053…` | Elixir 1.18.6, OTP 28.5.0.8 → 28.5.0.9 |
| `native-elixir-1-17` |  | `docker.io/library/elixir:1.17` | `08879d1eff52…` → `0dcd5012ce74…` | Elixir 1.17.3, OTP 27.3.4.19 → 27.3.4.20 |
| `native-elixir-1-19` |  | `docker.io/library/elixir:1.19` | `7e27df3e4a68…` → `c30251c267e3…` | Elixir 1.19.7, OTP 28.5.0.8 → 28.5.0.9 |
| `native-elixir-1-20` |  | `docker.io/library/elixir:1.20` | `1b850191faea…` → `028a755eef56…` | Elixir 1.20.5, OTP 29.1.2 → 29.1.3 |
| `native-go` |  | `docker.io/library/golang:1.27-bookworm` | `4fa7c968ba93…` → `ce59b41415e2…` | Go 1.27.4 (unchanged) |
| `native-hugo-image` |  | `ghcr.io/bex-co/bex-hugo:0.111.3` | `61d72075abce…` → `86115df682de…` | republished by `hugo-image.yml` run 45019148719; both platforms' single layer byte-identical, only the revision label differs |
| `native-node` |  | `docker.io/library/node:24-bookworm` | `9d266751ca3d…` → `49f7adaf9f15…` | Node 24.22.3 (unchanged) |
| `native-node-22` |  | `docker.io/library/node:22-bookworm` | `e13726ca6041…` → `7c400d78797d…` | Node 22.23.5 (unchanged) |
| `native-node-26` |  | `docker.io/library/node:26-bookworm` | `b7087bc4d973…` → `893ea48106e9…` | Node 26.13.1 (unchanged) |
| `native-python` |  | `docker.io/library/python:3.13-bookworm` | `cc55e4ded9c9…` → `ee1a30c36c5c…` | Python 3.13.18 (unchanged) |
| `native-python-3-11` |  | `docker.io/library/python:3.11-bookworm` | `726d304906fa…` → `9884c17ef6d3…` | Python 3.11.18 (unchanged) |
| `native-python-3-12` |  | `docker.io/library/python:3.12-bookworm` | `4209fae09a3e…` → `5f0cc9bc924d…` | Python 3.12.16 (unchanged) |
| `native-python-3-14` |  | `docker.io/library/python:3.14-bookworm` | `3fe825da0a80…` → `fbaf91204f27…` | Python 3.14.10 (unchanged) |
| `native-python-3-15` |  | `docker.io/library/python:3.15-bookworm` | `6f060cab421d…` → `2bf848386eab…` | Python 3.15.2 (unchanged) |
| `native-ruby` | yes | `docker.io/library/ruby:3.4-bookworm` | `d7c312a48736…` → `9d047dfbfd4d…` | Ruby 3.4.12 → 3.4.13 |
| `native-ruby-3-3` | yes | `docker.io/library/ruby:3.3-bookworm` | `ae098d019fd2…` → `e2bd2e75c554…` | Ruby 3.3.13 → 3.3.14 |
| `native-ruby-4-0` | yes | `docker.io/library/ruby:4.0-bookworm` | `862aacfe5415…` → `a74fe73b00b1…` | Ruby 4.0.9 → 4.0.10 |
| `native-rust` |  | `docker.io/library/rust:1-bookworm` | `ade39a710ef8…` → `75d26c23c24c…` | Rust 1.100.0 (unchanged) |

A second `resolve` at 13:37Z found only the two Paketo stacks moved again (their second release of the day), and the pins take that release. Paketo published that release's builder (`906ba1546346…`, lifecycle 0.21.30) by a third `resolve` at 15:45Z, which the adoption commit takes; that `resolve` found no other drift. The second `resolve` also ran the new line-version check (t008) against the registries: every line's `version` field equals the patch its image records.

Every official image above was rebuilt on the Debian root `@1795392000` (was `@1793827419`) and keeps its suite (bookworm, or trixie for Elixir 1.18–1.20); linux/amd64 and linux/arm64 are both published.

**All 25 move, not only #201's eight.** The monitor closes its issue only when no entry drifts (`issue_action`), so #201 can close by itself only once every entry matches, and the goal is every pinned image. The 17 entries beyond #201 are digest-only Debian rebuilds of unchanged runtimes, Elixir's OTP maintenance patches (the lines name only the OTP major) and the two bex republishes.

**#201 was reopened 2026-11-26.** GitHub closed it on 2026-11-25 through the closing keyword in the filing commit's message (`39e2cea8a9`, "… close #201"), not through a review or the monitor, so the DoD's "closed by the freshness run" could not be shown. The reopen comment says so.

## Parked — 2026-11-26 (the production check waits on the deploy)

**Shipped:** `9874e5ea21` (every drifted pin with its patch, the line-version check in `toolchain-freshness.py`, the table-driven Bun and Ruby tests) and `bef3a32083` (the common image from families run 45839946482, the rebuilt bex run image and the Paketo builder of the same stack). Details are in [t002](done/t002.md), [t003](done/t003.md), [t004](done/t004.md), [t005](done/t005.md) and [t008](done/t008.md).

**Met:**

- **DoD 1 at the review:** the dispatched freshness run [45854117852](https://github.com/bex-co/bex/actions/runs/45854117852) on `bef3a32083` validated the inventory and resolved all 56 entries with no drift, its new line-version check included. Drift newer than that opens a new issue, since #201 is closed.
- **DoD 2:** that run closed #201 at 15:57:50Z with "Upstream digests match the committed pins again." (`issue-action` close), after the reopen recorded in the review list.
- The Ruby profile and the Bun gate passed offline locally on the new lines, and the families run passed all 39 cases on the new common image (t005).

**Gate (clears by itself): a production deploy carrying `bef3a32083`.** Production's operator image carries the native pins (`native.go`, `nativebases.go`) and `BEX_CNB_BUILDER` only after the next batched deploy; Argo applies the kpack chart from main.

**Check that releases it (t006, then t009):**

1. A `chore(deploy): pin platform images to <sha>` on main whose `<sha>` descends from `bef3a32083`.
2. Signed in as the QA user (`bash scripts/qa-login.sh`), in a disposable workspace: a native Node service with `BUN_VERSION=1.4` logs `==> Using Bun 1.4.5 (from BUN_VERSION=1.4)`, and a native Ruby service with `.ruby-version` `3.4` logs `==> Using Ruby version 3.4.13 via /opt/render/project/src/.ruby-version` (a service created since 2026-11-19 with no signal logs `==> Using Ruby version 3.4.13 (default)`); both answer 200 at their `onbex.co` URLs. Delete both by ID and confirm `GET` answers 404.
3. The kpack ClusterBuilder is Ready on `builder-jammy-base@sha256:906ba1546346…` (`bex_build_clusterbuilder_ready` 1, `ClusterBuilderNotReady` not firing), with the ClusterStack on the 12:43:57Z stack and run image `bex-cnb-run@sha256:69aa695c7ce1…`.
4. Then t009.

**Follow-up for the next review (not this milestone's):** every push touching `lego/operator/internal/build/native*.go` republishes `ghcr.io/bex-co/bex-cnb-run:native-common-bookworm` (`native-base-image.yml` moves the tag after each build, whose revision label changes the digest), so `native-common-bookworm` reads as drift after almost any native change until the next families adoption, which is how it reached #201. Moving the tag only on a families run, or resolving that entry against a families-only tag, would leave the monitor reporting a newer qualified image only.
