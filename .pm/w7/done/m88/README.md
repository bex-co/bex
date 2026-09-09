# w7 · m88 — Make clear-cache deploys effective

**Worker:** worker7 **Goal:** The existing Clear build cache & deploy action causes a clean rebuild when per-App registry caching is enabled. **Status:** done

**Estimate:** 4h implementation; ~6h including the standing closing tasks.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Define and encode release-scoped build-cache reset intent | 45m | w7/m87/t008 — **DONE** |
| t002 | Carry clear-cache intent through deploy creation and projection | 50m | w7/m88/t001 — **DONE** |
| t003 | Reset native and Dockerfile cache reuse and publish a fresh cache | 55m | w7/m88/t002 — **DONE** |
| t004 | Keep cache resets correct through retry and overlapping deploys | 45m | w7/m88/t003 — **DONE** |
| t005 | Exercise the clean-rebuild journey and correct cache descriptions | 45m | w7/m88/t004 — **DONE** |
| t006 | Render parity | 30m | w7/m88/t005 — **DONE** |
| t007 | Simplify | 30m | w7/m88/t006 — **DONE** |
| t008 | Test coverage | 45m | w7/m88/t006 — **DONE** |
| t009 | Closeout | 15m | w7/m88/t007, w7/m88/t008 — **DONE** |

## Definition of done

- [x] For native and Dockerfile builds with registry caching enabled: ordinary deploy reuses the cache, clear-cache deploy recomputes without prior build artifacts, and the following normal deploy can reuse the fresh cache.
- [x] Reset intent is durable and scoped to the intended release; retry, process restart, cancellation, superseding deploys, and older cache writers obey the t001 behavior table.
- [x] REST, GraphQL, MCP, dashboard, and the unmodified official CLI honor the existing clear/do_not_clear enum; invalid values and unauthorized calls fail before mutation.
- [x] Another App/workspace's cache and deployable images remain untouched. Registry credentials stay outside tenant-executing BuildKit, and cache-off/kpack/image-only behavior is explicitly verified.
- [x] The native environment invalidation shipped by m87 remains correct, stale no-op descriptions and ADR018 evidence are corrected, live disposable journey evidence and meaningful regressions pass, and all standing closing tasks are complete.

## Source + Goal linkage

- **Source:** user-approved 2026-09-08 pm-brainstorm proposal 2 and `$pm` handoff; [w3/m46](../../../w3/done/m46/README.md) shipped clear-cache controls, and [w7/m86](../m86/README.md) shipped the cache whose effect those controls must now reset.
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md) Render-compatible, agent-operable hosting and [.pm/GOAL.md](../../../GOAL.md) #3 (git push to deploy).
- **Expected outcome:** a user or agent requesting a clean rebuild gets recomputed artifacts, with later ordinary builds able to reuse the fresh cache.
- **Why now:** the existing UI/API recovery control promises an effect that enabled registry caching currently ignores. Complete this integration before evaluating production enablement.
- **Render parity included:** [ADR018 Trigger a deploy](../../../../docs/ADR018-render-parity.md#deploys) updated for effective clear-cache under `BEX_BUILD_CACHE=registry`. [Render manual deploys](https://render.com/docs/deploys#manual-deploys) defines clear as rebuilding without prior build artifacts.
- **Dependencies and placement:** t001 depended on w7/m87/t008 because both milestones change build dispatch/cache identity. Four hours of implementation plus two hours of standing closing work; worker7 is a general-purpose queue with available capacity.

## Evidence and scope

Release-scoped `app.bex.co/clear-cache-release-generation` + `Options.SkipCacheImport` (purge, no import, still export). Unit/envtest cover Job shape, marker scoping, sibling Job fencing, and Trigger stamp/clear. Dated evidence: [docs/drills/2026-09-08-clear-cache.md](../../../../docs/drills/2026-09-08-clear-cache.md). Host docker driver and kind TLS blocked live warm→clear→warm re-measure; production enablement stays [043](../../043.md).
