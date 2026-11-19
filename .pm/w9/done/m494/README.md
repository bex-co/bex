# w9 · m494 — Persistent native runtime dependency caches across source-only rebuilds (Ruby `.gems`, Rust `.cargo`, Go `GOPATH`, Elixir `deps`/`_build`)

**Worker:** worker9 **Goal:** native Ruby, Rust, Go, and Elixir builds reuse their preconfigured `/opt/render/project/...` dependency directories across source-only rebuilds via version- and lockfile-keyed S3 archives, invalidating cleanly on toolchain upgrades and `clearCache: "clear"`. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Define per-runtime cache directory & invalidation-key map in `native.go` (Ruby `.gems`, Go `GOPATH/pkg/mod`, Rust `.cargo`, Elixir `deps`/`_build`) — **DONE** | 1h | w9/m493 |
| [t002](done/t002.md) | Wire runtime-dependency restore/export through the S3 build-cache phases with keyed metadata check, size cap, and `SkipCacheImport` purge — **DONE** | 1h30m | t001 |
| [t003](done/t003.md) | Unit & BuildKit integration tests: warm hit on source-only change, cold miss on runtime-version bump or lockfile change, corrupt-archive fallback, clear-cache reset — **DONE** | 1h | t002 |
| [t004](done/t004.md) | Production verification on at least two native runtimes (Ruby + Go or Rust) confirming warm dependency reuse and faster rebuilds — **DONE** | 30m | t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 30m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 20m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 45m | t005, t006 |
| [t008](done/t008.md) | Closeout — **DONE** | 10m | t007 |

## Definition of done

- On production: a Ruby, Go, Rust, or Elixir native service rebuilt after a source-only edit restores its prior runtime dependency directory without re-downloading/recompiling unchanged packages.
- Bumping the runtime version (e.g., Ruby 3.3 → 3.4, Go 1.24 → 1.25) or running `clearCache: "clear"` invalidates the cached runtime dependencies cleanly so ABI-incompatible compiled extensions never break the build or runtime.
- Cache miss, cap overflow, or corruption never fails a build; App deletion sweeps the cached archive.
- ADR018 (`Runtime, build strategy + Root Directory`) and ADR060 D3 record the per-runtime dependency cache behavior.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for one aspect of render parity` (Build Caching & Build Environment Parity, 2026-11-18, item 3); `lego/operator/internal/build/native.go:161-168`, [ADR060 § D3](../../../../docs/ADR060-build-worker-reliability-and-performance.md#d3--per-app-remote-build-cache-in-zot-registry-backend-modemax), and [w8/blocked/m161](../../../w8/blocked/m161/README.md).
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)) — matches Render's persistent native runtime dependency caching (`render.com/docs/environment-variables`, `render.com/docs/deploys`).
- **Expected outcome:** Native Ruby, Go, Rust, and Elixir rebuilds stop recompiling their entire dependency tree on every source commit, closing the largest remaining build-time gap vs Render.
- **Why now:** Builds directly on w9/m492's S3 build-cache store and w9/m493's named-context archive handoff; leaves Node's `node_modules` + `NODE_MODULES_CACHE` owned by `w8/blocked/m161` (which can adopt the same S3 store once unblocked).
- **Render parity:** included (t005). Matches Render's per-runtime persistent dependency cache semantics and updates ADR018 and ADR060.

## Unblock — 2026-11-19

- **Affected:** t004, then t008.
- **Source:** verified evidence, no user decision needed. The newest pin on `origin/main`, `a91e9aeb3` (`chore(deploy): pin platform images to dff52c596d13`), carries `98e143568`, the commit that adds `depscache.go` (`git merge-base --is-ancestor 98e143568 dff52c596d13` passes). It also carries w9/m492's `d7e76d312` and w9/m493's `7bfd497da`. Every `bex-system` Deployment on the production app cluster (`bex-controller-manager`, `bex-api` and the rest) runs `bex-operator@sha256:9850d3575465…`, the digest that pin commit writes.
- **Remaining gates:** none. Next: t004 as the QA user in `bex-canary`, then t008.

## Closeout — 2026-11-19

Every Definition-of-done bullet holds on production pin `dff52c596d13` ([t004](done/t004.md)):

- **Source-only rebuild.** A native Ruby service (Render's `render-examples/jekyll`, 34 gems) and a native Go service (Render's `render-examples/go-gin-web-server`, 15 modules) rebuilt after a source-only change restored their dependency directories. Ruby's `bundle install` went from 40.6 s cold, with three `with native extensions` lines, to 0.96 s with nothing installed. Go's build went from 44.9 s with 15 `go: downloading` lines to 0.78 s with none.
- **Invalidation.** `clearCache: "clear"` (`xdg purged`) installed everything again on both. A runtime change with the same `Gemfile.lock` (Ruby 3.3.5 → 3.3.0, both under `.gems/ruby/3.3.0`) restored the archive but dropped its gems, and all 34 gems installed again with their extensions. Every build of the two fixtures went live and served 200.
- **Never fails a build; deletion sweeps it.** A miss built cold. The cap and a corrupt archive were proven by the BuildKit drill and the unit tests ([ADR060 D3 w9/m494 amendment](../../../../docs/ADR060-build-worker-reliability-and-performance.md#d3-amendment-w9m494--native-dependency-directories-across-builds)), on w9/m493's restore path, which production already proved with a corrupt archive. Deleting the fixtures emptied their `bex-build-cache` prefixes.
- **Docs.** ADR018's `Runtime, build strategy + Root Directory` row and ADR060 D3's w9/m494 amendment record the behavior and this production proof.
