# w6 · m291 — Build toolchain digests are fresh again and #105 closes

**Worker:** worker6 **Goal:** every pinned build-toolchain image is at a reviewed current digest, apps still build on the new toolchain, and the freshness monitor's issue #105 closes. **Status:** blocked (t001 done, PR #106 closed; pins gated on w8/m91 D3 custody; the runner-link gate is gone since 2026-11-09)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Re-resolve the 25 drifted build-toolchain entries and verify their manifests](done/t001.md) — **DONE** | 45m | — |
| t002 | [Update every pinned file and the freshness inventory](t002.md) | 45m | t001 |
| t003 | [Qualify the new toolchain: build hello-go and one Node or Python buildpack app on dev-6](t003.md) | 1h | t002 |
| t004 | [Close PR #106 with the reason and confirm the monitor closes #105](t004.md) | 20m | t002 |
| t005 | [Simplify](t005.md) | 15m | t003, t004 |
| t006 | [Test coverage](t006.md) | 20m | t005 |
| t007 | [Closeout](t007.md) | 10m | t006 |

## Definition of done

- `toolchain-freshness.json` and every pinned file carry the reviewed digests; the freshness workflow's next run reports no drift and #105 is closed.
- hello-go and one Node or Python buildpack app build and serve on dev-6 with the new toolchain; after deploy, a production build succeeds.
- PR #106 is closed with an explanation.

## t001 resolve (2026-11-08, `toolchain-freshness.py resolve` at `1706435e3`)

`validate` passes. 25 of the 47 entries have moved upstream (committed → observed):

| id | upstream | committed | observed |
| --- | --- | --- | --- |
| cnb-builder | `docker.io/paketobuildpacks/builder-jammy-base:latest` | `bd03726c07e9…` | `7192795369e3…` |
| cnb-run-image | `ghcr.io/bex-co/bex-cnb-run:latest` | `c737a6661e03…` | `6f5c2f7b8f1c…` |
| cnb-stack-build | `docker.io/paketobuildpacks/build-jammy-base:latest` | `2492a2d7dd93…` | `a66123565724…` |
| cnb-stack-run | `docker.io/paketobuildpacks/run-jammy-base:latest` | `8a671e0f8a5b…` | `dc8dd7029da5…` |
| native-busybox | `docker.io/library/busybox:1.37.0` | `bdf57e528e45…` | `a7dbc319b143…` |
| native-common-bookworm-source | `docker.io/library/buildpack-deps:bookworm` | `db73e309a26d…` | `f53cd4c58cdb…` |
| native-elixir | `docker.io/library/elixir:1.18` | `17e900a56572…` | `16382709b376…` |
| native-elixir-1-17 | `docker.io/library/elixir:1.17` | `0ff6cbfc498e…` | `08879d1eff52…` |
| native-elixir-1-19 | `docker.io/library/elixir:1.19` | `8f4027522705…` | `7e27df3e4a68…` |
| native-elixir-1-20 | `docker.io/library/elixir:1.20` | `c904ce753c35…` | `1b850191faea…` |
| native-go | `docker.io/library/golang:1.27-bookworm` | `d9b6a1beb947…` | `dc0963f32053…` |
| native-node | `docker.io/library/node:24-bookworm` | `d862bb5d082c…` | `8fa96264f302…` |
| native-node-22 | `docker.io/library/node:22-bookworm` | `18a6d90fd605…` | `7702d8aa4d59…` |
| native-node-26 | `docker.io/library/node:26-bookworm` | `3b764117159b…` | `a546d3f8acd2…` |
| native-pnpm | `registry.npmjs.org/pnpm:latest-10` | `c729f6538e00…` | `7e8a946108e7…` |
| native-pnpm-linux-arm64 | `registry.npmjs.org/@pnpm/linux-arm64:latest-10` | `e5f004bc5e0b…` | `555753d2d2df…` |
| native-pnpm-linux-x64 | `registry.npmjs.org/@pnpm/linux-x64:latest-10` | `0ccd4ab9ffee…` | `bd473ea50a48…` |
| native-python | `docker.io/library/python:3.13-bookworm` | `3856e3e7bf4d…` | `cc55e4ded9c9…` |
| native-python-3-11 | `docker.io/library/python:3.11-bookworm` | `84fa927127b4…` | `726d304906fa…` |
| native-python-3-12 | `docker.io/library/python:3.12-bookworm` | `a5bab0ff300f…` | `4209fae09a3e…` |
| native-python-3-14 | `docker.io/library/python:3.14-bookworm` | `a8cf203c684b…` | `3fe825da0a80…` |
| native-ruby | `docker.io/library/ruby:3.4-bookworm` | `8853c6f24866…` | `d7c312a48736…` |
| native-ruby-3-3 | `docker.io/library/ruby:3.3-bookworm` | `0e4ba04b135a…` | `ae098d019fd2…` |
| native-ruby-4-0 | `docker.io/library/ruby:4.0-bookworm` | `470eb587e87c…` | `862aacfe5415…` |
| native-rust | `docker.io/library/rust:1-bookworm` | `9181b56c2a21…` | `e6f2cff35d0f…` |

These pins live in 8 files: `lego/operator/internal/build/native.go` (17), `nativepnpm.go` (3), `deploy/gitops/charts/kpack/platform.yaml` (3), `deploy/native-common/Dockerfile` (2), `deploy/native-base/Dockerfile`, `deploy/cnb-run/Dockerfile`, `lego/operator/cmd/manager/main.go` and `lego/operator/config/manager/manager.yaml`.

## BLOCKED — 2026-11-08: the inventory and native inputs are custody-pinned by w8/m91

**Gate 1 (w8 / user), the same gate as `w6/blocked/m163`:**

- `.github/scripts/native-common-qualify.py` lists `lego/operator/internal/build/toolchain-freshness.json` in `IMAGE_POLICY_FILES`, and `deploy/native-common/Dockerfile` plus the `native-base` inputs in `FACTORY_PATHS`.
- `image_inputs_compatible` (`:181`) accepts the retained `bex-cnb-run@sha256:0941…` candidate only when every image input is byte-identical, with one exception: the recorded `D3_LEGACY_0941` workflow transition.
- Every refresh must edit `toolchain-freshness.json`, because it carries the `committed` and `resolved_at` fields that `validate` requires. So even a CNB-only refresh invalidates the candidate w8 is transitioning.
- **Release:** w8 re-pins or retires the 0941 transition (m163's gate), or approves a fresh native-common qualification.

**Gate 2 (user), the runner link:**

- Native pin edits trigger `native-common-image.yml` (paths `lego/operator/internal/build/native*.go`).
- Its builds already fail on the degraded link: run on `34702afaa` (2026-11-08) died pushing to GHCR with `DeadlineExceeded`; issue #133 and `w6/blocked/m158`.
- **Release:** a normal link.

**Done so far:**

- **t001:** the table above.
- **t004, PR half:** PR #106 is closed with an explanation (outside contributor; it was built on the pre-rewrite base).
- **Still to do:** t002, t003, t004's #105 half, and t005–t007.

**When unblocked:**

1. Re-run `resolve`, since the digests above will have moved again.
2. Apply the pins and `resolved_at`.
3. Qualify on dev-6 (t003).
4. Let native-common re-qualify with the new inputs.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w6` 2026-11-08, proposal 2; issue #105 (open since 2026-10-22, 25 entries on 2026-11-05).
- **Goal linkage:** ADR008 in-cluster builds — reliable and secure builds (the CNB builder drift includes CVE fixes; ADR060 D7).
- **Expected outcome:** customer builds use current, reviewed toolchain images and the monitor's review queue is empty.
- **Why now:** the queue has grown for 17 days, partial refreshes (`93bc3ef2e`, `ee7e3eaa7`) left it open, and the only fix attempt (PR #106) is unmergeable.
- **Render parity:** omitted — build toolchain pins only, no REST/GraphQL/MCP/UI change.

## Re-triage — 2026-11-12 (w6/119)

- **Gate 2, the runner host's link (#133), is gone.** bex CI runs on GitHub-hosted runners since the user's 2026-11-09 decision (DO_NOT_DO #CI-RUNNERS), so the native-common rebuild no longer waits on the Mac's link.
- **Gate 1 stands.** `toolchain-freshness.json` and the native inputs are byte-pinned by w8/m91's D3 custody (`IMAGE_POLICY_FILES` in `.github/scripts/native-common-qualify.py`), so re-pinning them is w8's to accept.
