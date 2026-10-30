# Native tool supply — C selected by parity rule (reviewed 2026-10-29)

**Gate for:** m91, m93, m94, m95, m96, m97, m98, m99, m100, m101, m102, m103, m107. All 13 milestones ask native builds and runtimes to carry a tool from [Render's native tools table](https://render.com/docs/native-runtimes#tools-and-utilities). The table says "all native runtimes include the tools listed below" (Debian 12 bookworm). Render's alternative for anything missing is "deploy with Docker instead."

| Milestone | Render tool(s) |
| --- | --- |
| m91 | `postgresql-client` (plus a PG 18 client: bex's default server is PG 18, and `pg_restore` must be at least the dump's version) |
| m93 | `pandoc` |
| m94 | `sqlite3` |
| m95 | `libvips-dev`, `libvips-tools` |
| m96 | `gettext` |
| m97 | `ghostscript` |
| m98 | `jq`, `rsync` |
| m99 | `typescript` |
| m100 | `swig` |
| m101 | `webpack` |
| m102 | `pigz` (build only) |
| m103 | `python3-dev`, `python3-pip`, `python3-setuptools` |
| m107 | `zip` |

## What the bases have

bex builds on the 16 reviewed official toolchain images:

- Node 22/24/26
- Python 3.10–3.14
- Ruby 3.3/3.4/4.0
- Elixir 1.17–1.19 (1.18/1.19 are trixie)
- Go 1.24
- Rust 1

All are `buildpack-deps`-based. Measured on `node:24-bookworm`:

- **Already present:** `imagemagick`, `unzip`, `wget`, `curl`, `git`, `make`, `gcc`, `g++`
- **bex supplies itself:** `ffmpeg` (w8/m82), Yarn and pnpm
- **Missing everywhere:** `gettext`, `ghostscript`, `jq`, `libvips-dev`/`-tools`, `pandoc`, `pigz`, `postgresql-client`, `python3-dev`/`-pip`/`-setuptools`, `rsync`, `sqlite3`, `swig`, `zip`, `nano`, `vim`, `gnupg2`, `typescript`, `webpack`

The FFmpeg pattern does not generalize. FFmpeg is one fully static binary copied with a byte-reproducible `COPY --link`. Most of these tools are instead Debian packages that need:

- **shared-library closures:** `psql` needs `libpq`; `gs` needs `libgs`
- **data trees compiled for `/usr`:** Ghostscript resources, pandoc data, vips modules
- **libraries the app's own process loads:** pyvips `dlopen`s `libvips.so.42`
- **headers matching a specific Python:** for `python3-dev`

TypeScript 7 also ships per-platform native packages, and Webpack carries a large dependency graph.

## Options

**A — `apt-get install` inside every native build.** Measured on `node:24-bookworm` for the missing set: **+242 s on each cold build and about +550 MB per image**.

- **Pros:** the simplest code, and the same Debian packages Render has.
- **Per-app cost:** BuildKit caches per App (`A_cache`) and the RUN layer is not byte-reproducible. Every App stores its own copy, every node pulls each App's copy, and every cold build depends on Debian and PGDG mirrors.

**B — one bex-built relocatable tools image**, copied into every native image under `/opt/bex-tools` with RPATH-patched binaries. Like FFmpeg, this is one shared byte-identical layer and one CI image.

- **Fragile:** plugin and data paths are compiled for `/usr`.
- **Unsolved:** `libvips` must load inside the tenant's own Python or Ruby process, which needs global loader configuration. That configuration can shadow base libraries on the trixie lines.

Not recommended.

**C — per-line bex native base images (recommended).** A workflow shaped like `cnb-run-image.yml` (publish, never pin) builds, for each reviewed line, `FROM <reviewed official base digest>` plus:

- `apt-get install` of Render's tools table;
- a PGDG `postgresql-client-18`;
- `npm install -g typescript webpack webpack-cli` where Node is present.

It pushes both architectures to `ghcr.io/bex-co/bex-native:<runtime>-<line>`. The operator then pins those digests in `nativeToolchains`, with the official digest kept as the inventory's upstream. The pin is a reviewed commit under ADR060 D7.

- **Pros:**
  - dpkg-correct, with libraries, headers and data where Render has them;
  - one shared layer per line, stored once and pulled once per node;
  - no network during tenant builds;
  - every Render tool is supplied at once.
- **Costs:**
  - **CI time:** 16 images × 2 architectures on the ARM64 self-hosted runners. amd64 runs under QEMU, and production is amd64, so it can't be skipped. Expect tens of minutes to hours of runner time per toolchain refresh.
  - **Size:** about +550 MB per line image.
  - **Two-commit bumps:** every toolchain bump becomes publish-then-pin.

**D — no blanket supply.** Record each tool as a divergence ("use a Dockerfile"), as Render advises for tools outside its table. The 13 milestones would close as divergences.

## Historical decision request (superseded by the 2026-10-29 review)

Choose A, B, C or D. For C, this also approves:

- adding the publishing workflow;
- its runner time;
- about +550 MB per native line image.

With C or A approved, the 13 milestones' implementation and per-tool regression tasks proceed. Each milestone's own hosted acceptance task then follows its rollout.

## 2026-10-29 复核结论

采用 C；来源为现有 parity 规则和工程判断，不是用户新授权。上述 A/B/C/D 是历史对比。新增发布工作流、共享镜像和版本锁定属于实现工作，不再作为用户选型门槛。m91 先建立供应机制，m93–m103、m107 复用；每项仍须分别满足原 DoD。不得据此新增云资源或提交/push。

## Implementation review — 2026-10-29

The live table now has 27 reviewed primary lines, including Bullseye, Bookworm and Trixie. The supplier keeps the official image references for version/companion selection and places reviewed derivative references in `nativeSuppliedBases`; exact primary trees and verified image environment are restored before companion installation. Exact OTP 28.0 is Bookworm and selects a Bookworm derivative even when its Elixir line normally uses Trixie. A generated-image gate exercised the actual installed release, Elixir, clients, libraries and offline runtime; legacy-history execution remains separately bounded.

Publish candidates as `ghcr.io/bex-co/bex-cnb-run:native-<line>-<commit>`. This reuses the existing public build-plane package, whose anonymous pull was verified, and never moves its CNB `latest` tag. New GHCR packages start private and this session has no package-visibility API or connected browser; distinct public tags supply the same reviewed artifacts without an external console gate or a new credential. This is an engineering routing choice under the existing parity decision, not a new user product decision. Pin only after the per-line multiarch checks, signature/SBOM review and anonymous pull pass.
