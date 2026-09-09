# w7 · m88 — Make clear-cache deploys effective

**Worker:** worker7 **Goal:** The existing Clear build cache & deploy action causes a clean rebuild when per-App registry caching is enabled. **Status:** todo

**Estimate:** 4h implementation; ~6h including the standing closing tasks.

**Next:** t001 after w7/m87/t008 (verified milestone closeout).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Define and encode release-scoped build-cache reset intent | 45m | w7/m87/t008 |
| t002 | Carry clear-cache intent through deploy creation and projection | 50m | w7/m88/t001 |
| t003 | Reset native and Dockerfile cache reuse and publish a fresh cache | 55m | w7/m88/t002 |
| t004 | Keep cache resets correct through retry and overlapping deploys | 45m | w7/m88/t003 |
| t005 | Exercise the clean-rebuild journey and correct cache descriptions | 45m | w7/m88/t004 |
| t006 | Render parity | 30m | w7/m88/t005 |
| t007 | Simplify | 30m | w7/m88/t006 |
| t008 | Test coverage | 45m | w7/m88/t006 |
| t009 | Closeout | 15m | w7/m88/t007, w7/m88/t008 |

## Definition of done

- [ ] For native and Dockerfile builds with registry caching enabled: ordinary deploy reuses the cache, clear-cache deploy recomputes without prior build artifacts, and the following normal deploy can reuse the fresh cache.
- [ ] Reset intent is durable and scoped to the intended release; retry, process restart, cancellation, superseding deploys, and older cache writers obey the t001 behavior table.
- [ ] REST, GraphQL, MCP, dashboard, and the unmodified official CLI honor the existing clear/do_not_clear enum; invalid values and unauthorized calls fail before mutation.
- [ ] Another App/workspace's cache and deployable images remain untouched. Registry credentials stay outside tenant-executing BuildKit, and cache-off/kpack/image-only behavior is explicitly verified.
- [ ] The native environment invalidation shipped by m87 remains correct, stale no-op descriptions and ADR018 evidence are corrected, live disposable journey evidence and meaningful regressions pass, and all standing closing tasks are complete.

## Source + Goal linkage

- **Source:** user-approved 2026-09-08 pm-brainstorm proposal 2 and `$pm` handoff; [w3/m46](../../w3/done/m46/README.md) shipped clear-cache controls, and [w7/m86](../done/m86/README.md) shipped the cache whose effect those controls must now reset.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) Render-compatible, agent-operable hosting and [.pm/GOAL.md](../../GOAL.md) #3 (git push to deploy).
- **Expected outcome:** a user or agent requesting a clean rebuild gets recomputed artifacts, with later ordinary builds able to reuse the fresh cache.
- **Why now:** the existing UI/API recovery control promises an effect that enabled registry caching currently ignores. Complete this integration before evaluating production enablement.
- **Render parity included:** [ADR018 Trigger a deploy](../../../docs/ADR018-render-parity.md#deploys) is currently ✅ across all four surfaces but still carries the obsolete no-cache rationale. [Render manual deploys](https://render.com/docs/deploys#manual-deploys) explicitly defines clear as rebuilding without prior build artifacts.
- **Dependencies and placement:** t001 depends on w7/m87/t008, its verified closeout, because both milestones change build dispatch/cache identity. Four hours of implementation plus two hours of standing closing work; worker7 is a general-purpose queue with available capacity.

## Evidence and scope

The code-backed gap is in TriggerParams.ClearCache and validateTrigger in lego/backend/internal/deploys/service.go: the enum is accepted and validated as a no-op, while lego/operator/internal/build/build.go imports a cache when enabled. w3/m46's controls and m86's cache are completed work; this milestone repairs their interaction. The latest m86 production-setting evidence is dated 2026-09-05 and records the gate off; this is not a newly observed production incident.

Use the existing deploy controls and shared business verb, the types contract, and operator-owned build mechanism. Keep credentials isolated and reset scope per App/release. No cluster-wide prune, first-party CLI, new cache service, persistent cache volumes, or production cache setting change. The follow-up [043](../043.md) is a measured enablement decision.
