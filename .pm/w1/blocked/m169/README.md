# w1 · m169 — Buildpack Node services start on current Node: a run image with libatomic

**Worker:** worker1 **Goal:** A `builder: buildpack` Node app with an open `engines.node` range (Render resolves it to the latest Node) builds and then starts. Today it crash-loops on `libatomic.so.1`. **Status:** blocked (t001, t002 done; t003 needs the first `cnb-run-image.yml` publish after ship)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | bex CNB run image recipe: `run-jammy-base` + `libatomic1`, with a Node 26 start check — **DONE** | 30m | — |
| t002 | Publish workflow `cnb-run-image.yml`, plus `deploy/cnb-run/Dockerfile` as a reviewed pin site — **DONE** | 45m | t001 |
| t003 | Pin the published digest as the kpack ClusterStack run image (ADR060 D7) | 30m | t002 |
| t004 | Deploy + live verify: `examples/hello-node` (`>=20` ⇒ Node 26) serves 200 on production | 30m | t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 20m | t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- The production kpack `ClusterStack bex-jammy-base` names `ghcr.io/bex-co/bex-cnb-run@sha256:…` as its run image. The digest is recorded under inventory id `cnb-run-image`, and `bash scripts/build-toolchain-freshness.sh validate` passes.
- A `builder: buildpack` build of `examples/hello-node` (`engines.node: ">=20"`) selects Node 26 and serves `200` on production.
- The regression check is enforced: `cnb-run-image.yml` fails if the official Node 26 binary cannot start on the pushed digest for either platform, and the Dockerfile's `ldconfig` gate fails the image build itself.

## Triage (2026-10-02)

- **Render parity rules out (b).** Render's Node version docs (render.com/docs/node-version) say an unbounded range "such as `>=20` always resolves to the latest release of Node.js". Render defaults to `24.21.0` only when nothing pins a version. Defaulting `BP_NODE_VERSION` to an LTS for open ranges would therefore diverge. The parity answer is that the latest Node must _run_.
- **`run-jammy-full` does not fix it (option (a) as written).** The current `paketobuildpacks/run-jammy-full` amd64 layers contain no `libatomic` file. A build that starts the official `node-v26.10.0-linux-x64` binary on it fails with `error while loading shared libraries: libatomic.so.1`, exactly as on `run-jammy-base@sha256:33c1e0d7…`. Paketo's `jammy-full-stack` run package list has no `libatomic1` either. The fix is therefore a bex-extended run image: `run-jammy-base` plus the single 10 kB `libatomic1` package.
- **Proved locally.** `deploy/cnb-run/Dockerfile` built for linux/amd64 and linux/arm64, then `deploy/cnb-run/node-check.Dockerfile` ran against each image:
  - On the bex image, Node `v26.10.0 x64 starts` and `v26.10.0 arm64 starts`.
  - On stock `run-jammy-base`, the same check exits 127 on `libatomic.so.1`.
  - The bex image keeps `User 1002:1000` and `io.buildpacks.stack.id=io.buildpacks.stacks.jammy`, which is what kpack matches against the build image.
- **Tenant workaround until this ships:** set `BP_NODE_VERSION=24.*` on the service. That only works once `w1/121` (`blocked/121.md`) deploys; before it, service env vars never reached the kpack build.

## Source + Goal linkage

- **Source:** `.pm/w1/120.md` (now `done/120.md`), from `w1/108`'s production validation 2026-10-02 (fixture `qa-20261002-108-bp`), promoted during `/loopx w1` the same day.
- **Goal linkage:** ADR008 / ADR018 Render parity for the zero-command buildpack path. Render runs the latest Node for an open range, and bex must too.
- **Expected outcome:** the most common buildpack input, a Node app with `engines.node: ">=20"`, serves instead of crash-looping, with no tenant-side `BP_NODE_VERSION` pin needed.
- **Why now:** every new Node major from 25 on links `libatomic.so.1`, so the failure is the default outcome for open ranges and gets more common over time. The next builder bump cannot be validated green without it.
- **Render parity task omitted:** the change is in the build plane's run image only, with no REST/GraphQL/MCP/dashboard surface change.
