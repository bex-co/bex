# w5 · m134 — Buildpack releases build on their App's cache, and a clear starts a fresh one

**Worker:** worker5 **Goal:** A buildpack (kpack) release restores the layers its App's previous release cached, from one tag in the App's own image repository, and "Clear build cache & deploy" starts a fresh cache that later releases build on. **Status:** done

## Tasks (in order)

| id   | title                                                          | est | depends_on |
| ---- | -------------------------------------------------------------- | --- | ---------- |
| t001 | One kpack cache tag per App, in the App's image repository — **DONE** | 40m | —          |
| t002 | A clear-cache release starts a fresh kpack cache lineage — **DONE** | 45m | t001       |
| t003 | Render parity: clearCache on every surface for buildpack apps — **DONE** | 20m | t002       |
| t004 | Simplify — **DONE** | 15m | t003       |
| t005 | Test coverage — **DONE** | 30m | t003       |
| t006 | Closeout — **DONE** | 10m | t005       |

## Definition of done

- Two consecutive buildpack releases of one App configure the same kpack cache tag, so the second restores what the first cached. Two Apps never share a tag, including namesakes in different workspaces and an App named `<app>-cache`.
- No per-release cache tag lands in the image repository: the App's one cache tag takes one of Zot's five retained tags, so four release images stay for rollback (about two before).
- A clear-cache release builds without the App's earlier cache, and the ordinary releases after it build on the cache it saved, never on the cleared one.
- ADR060 (context and D3) and ADR018 describe kpack's cache and what `clearCache: clear` does for a buildpack service.

## Decisions

- **Image repository, not `<repo>-cache` (t001, 2026-10-07).** The quality review found that `<repo>-cache` is also the image repository of an App named `<app>-cache` in the same workspace. That collision dates from w7/m86 and is filed as w5/150, with data loss on delete. An ungated kpack cache there would have filled that App's retention, or failed while the other App held the repository's access rule. The App's own image repository needs no new grant. A tag left by a clear ages out with old release tags. The cost is one retention slot.
- **Record the clear before the build starts (t002).** bex-api drops the clear marker on the next deploy, so the operator writes `status.buildCacheGeneration` before it dispatches the clear release's build. It does not rely on a later phase write.

## Source + Goal linkage

- **Source:** promoted from inbox w5/137, found by w5/129's efficiency review (2026-10-07).
  - `lego/operator/internal/build/kpack.go` `KpackImage` sets `cache.registry.tag` to `o.KpackImageRef() + "-cache"`, which is `<repo>:gen-N-cache`. Each release's Image builds once, so its cache is never read back.
  - Found while triaging: those tags sit in the image repository. Zot keeps the five most recently pushed tags per repository (`registry/zotconfig.go`), so a buildpack service keeps about two releases' images for rollback.
  - The BuildKit path's clear purges one per-App `:cache` tag (w7/m88). kpack reads and writes a single tag, so a clear needs a new tag that later releases remember. bex-api drops the clear marker on the next ordinary deploy, so that memory must be operator-owned App status.
- **Goal linkage:** ADR060 D3 (build cache) and the build-speed pillar of ADR008.
- **Expected outcome:** buildpack deploys after the first restore their dependency and toolchain layers instead of rebuilding them, and rollback reaches as far back as on Dockerfile services.
- **Why now:** w5/129 retired spent revisions, so one App now has one live kpack Image, and a per-App cache tag has no concurrent writer under ADR060 §D1a.
- **Render parity included:** `clearCache: clear` changes meaning for buildpack services on REST, GraphQL, MCP and the dashboard's deploy menu. ADR018 says kpack accepts the enum "without inventing unsupported cache effects".

## Result (2026-10-07)

- **Cache tag (t001):** `build.Options.KpackCacheRef` puts every kpack Image of an App on one cache tag in its image repository, `<repo>:kpack-cache`, on the host kpack pushes to. It replaces the per-release `<repo>:gen-N-cache`. One `kpackRegistry()` now picks that host for the image tag, the cache tag, the kpack registry credential and `canonicalKpackImage`.
- **Clear (t002):** a clear-cache release records `status.buildCacheGeneration`, a new App status field (CRD regenerated), before its build starts. Its kpack cache moves to `kpack-cache-gen-<release>`, which later releases keep once the marker is gone. A tag the App leaves ages out with old release tags.
- **Parity (t003):** REST, GraphQL and MCP reach `clear` through `deploys.Service.Trigger`'s `stampClearCacheRelease`. The dashboard's "Clear build cache & deploy" shows for every repo-backed service. So buildpack services get the new clear on every surface. ADR018's trigger row and ADR060's clear-cache table say kpack honors it, whatever `BEX_BUILD_CACHE`.
- **Simplify (t004):** three reviews. They moved the cache out of `<repo>-cache` (see Decisions), made the clear persist before dispatch, shared the kpack test helpers with w5/129's prune test, and folded the kpack-host fallbacks into one helper.
- **Tests (t005):**
  - `build.TestKpackReleasesShareTheirAppsCache`: one exact tag across releases. It is distinct for another App, for an App named `hello-cache` and for a namesake in another workspace. It follows the kpack host alias, and a clear moves to a new tag that later releases keep.
  - `controller.TestAClearStartsABuildpackCacheTheNextReleaseKeeps`: ordinary, clear, then ordinary releases through the reconciler. The clear's tag differs, is recorded before its Image is created, and release 4 keeps it.
  - 8 mutants fail them, including the shipped per-release tag and a clear recorded only after its build starts.
- **Docs:** ADR060's context line, its clear-cache table and "kpack's cache (w5/m134)"; ADR018's trigger row.
- **Gates:** operator `make test`; `make lint` (all four modules).
- **Not verified live:** no kpack build ran against a real registry. The tests cover the Image spec and the controller's lineage.
- **Follow-up:** w5/150, an App named `<app>-cache` that shares the App's BuildKit cache repository and loses its images when the App is deleted.
